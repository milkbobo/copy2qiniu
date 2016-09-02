package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"qiniupkg.com/api.v7/conf"
	"qiniupkg.com/api.v7/kodo"
	"qiniupkg.com/api.v7/kodocli"
	"strconv"
	"strings"
	"time"
)

var (
	AccessKey             string
	SecretKey             string
	Bucket                string
	DomainName            string
	OriginPath            string
	OriginAbsolutePath    string
	TargetPath            string
	RunPath               string
	IsRefreshFile         string
	RefreshFileRetryTimes = 1
	FailRefreshPath       string
)

type PutRet struct {
	Hash string `json:"hash"`
	Key  string `json:"key"`
}

func init() {
	//获取运行目录地址
	var err error
	RunPath, err = filepath.Abs("./")
	if err != nil {
		checkError("filepath.Abs 出错内容:" + err.Error())
	}

	FailRefreshPath = RunPath + "/failRefresh.config.temp"
}

func main() {

	//获取运行程序下的配置文件
	config, error := readFile(RunPath + "/" + "copy2qiniu.config.json")
	checkError(error)

	//把json配置文件解析
	error = getConfig(config)
	checkError(error)

	//读取文件夹下所有文件，除了隐藏文件意外
	DirInfo, error := readDir(OriginAbsolutePath)
	checkError(error)

	//获得需要上传的文件
	uploadFileData, uploadHashFileData, error := getFileInfo(DirInfo)
	checkError(error)

	//上传文件
	error = updataFile(uploadFileData)
	checkError(error)

	//更新七牛文件的缓存
	error = refreshFile(uploadHashFileData)
	checkError(error)

	fmt.Println("程序执行完毕，谢谢使用。")

}

func readFile(path string) (string, string) {

	fi, err := os.Open(path)
	if err != nil {
		return "", "os.Open 出错内容:" + err.Error()
	}
	defer fi.Close()

	fd, err := ioutil.ReadAll(fi)
	if err != nil {
		return "", "ioutil.ReadAll 出错内容:" + err.Error()
	}
	return string(fd), ""
}

func getConfig(jsonData string) string {
	configData := map[string]string{}
	err := json.Unmarshal([]byte(jsonData), &configData)
	if err != nil {
		return "json.Unmarshal 出错内容:" + err.Error()
	}

	var ok bool

	AccessKey, ok = configData["AccessKey"]
	if AccessKey == "" || ok == false {
		return "AccessKey配置参数不存在或者不能为空"
	}

	SecretKey, ok = configData["SecretKey"]
	if SecretKey == "" || ok == false {
		return "SecretKey配置参数不存在或者不能为空"
	}

	Bucket, ok = configData["Bucket"]
	if Bucket == "" || ok == false {
		return "Bucket配置参数不存在或者不能为空"
	}

	OriginPath, ok = configData["OriginPath"]
	if OriginPath == "" || ok == false {
		return "OriginPath配置参数不存在或者不能为空"
	}

	TargetPath, ok = configData["TargetPath"]
	if TargetPath == "" || ok == false {
		return "TargetPath配置参数不存在或者不能为空"
	}

	IsRefreshFile, ok = configData["IsRefreshFile"]
	if IsRefreshFile == "" || ok == false {
		return "IsRefreshFile配置参数不存在或者不能为空"
	}

	DomainName, ok = configData["DomainName"]
	if DomainName == "" || ok == false {
		return "DomainName配置参数不存在或者不能为空"
	}

	conf.ACCESS_KEY = AccessKey
	conf.SECRET_KEY = SecretKey

	//获取绝对路径
	OriginAbsolutePath, err = filepath.Abs(OriginPath)
	if err != nil {
		checkError("filepath.Abs 出错内容:" + err.Error())
	}

	return ""

}

func combineDirInfo(a1 []string, a2 []string) []string {
	for _, value := range a2 {
		if len(value) != 0 {
			a1 = append(a1, value)
		}
	}
	return a1
}

func readDir(path string) ([]string, string) {
	result := []string{}
	//获取文件夹所有的文件列表
	tempResult, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, "ioutil.ReadAll 出错内容:" + err.Error()
	}
	for _, singleFileInfo := range tempResult {

		//如果是隐藏文件，就忽略
		if singleFileInfo.Name()[0:1] == "." || singleFileInfo.Name() == "copy2qiniu.config.json" {
			continue
		}

		name := path + "/" + singleFileInfo.Name()

		if singleFileInfo.IsDir() {
			//发现dir
			result2, err := readDir(name)
			if err != "" {
				return nil, err
			}
			result = combineDirInfo(result, result2)
		} else {
			result = append(result, name)
		}
	}
	return result, ""

}

func writeFile(filePath string, data string) {
	openFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0666)
	defer openFile.Close()
	if err != nil {
		checkError("os.OpenFile 出错内容:" + err.Error())
	}

	_, err = io.WriteString(openFile, data)
	if err != nil {
		checkError("io.WriteString 出错内容:" + err.Error())
	}
}

func getFileInfo(Files []string) ([]string, []string, string) {

	uploadFileData := []string{}
	uploadHashFileData := []string{}

	//new一个Bucket管理对象
	c := kodo.New(0, nil)
	p := c.Bucket(Bucket)

	for _, singleFile := range Files {

		//调用Stat方法获取文件的信息
		relativePathFileName := strings.Replace(singleFile, OriginAbsolutePath, "", -1)[1:]
		severFileName := TargetPath + relativePathFileName
		entry, err := p.Stat(nil, severFileName)
		// entry, err := p.Stat(nil, "yourdefinekey")

		//打印出错时返回的信息
		if err != nil {
			if err.Error() == "bad token" {
				return nil, nil, "token错误"
			}

			if err.Error() == "no such file or directory" {
				uploadFileData = append(uploadFileData, singleFile)
				continue
			}

		}

		//获取本地文件的hash值
		hashData, err := GetEtag(singleFile)
		if err != nil {
			return nil, nil, "GetEtag 出错内容:" + err.Error()
		}

		//如果hash值不同，也添加到上传队列中
		if entry.Hash != hashData {
			uploadHashFileData = append(uploadHashFileData, relativePathFileName)
			uploadFileData = append(uploadFileData, singleFile)
			continue
		} else {
			fmt.Println(`不上传该文件,"` + relativePathFileName + `"和服务器上的"` + severFileName + `"的hash值一样。`)
		}

		if err != nil {
			return nil, nil, err.Error()
		}
	}

	return uploadFileData, uploadHashFileData, ""

}

func updataFile(updataFileData []string) string {

	for _, singleFile := range updataFileData {

		relativePathFileName := strings.Replace(singleFile, OriginAbsolutePath, "", -1)[1:]

		//创建一个Client
		c := kodo.New(0, nil)

		updataFileName := TargetPath + relativePathFileName

		//设置上传的策略
		policy := &kodo.PutPolicy{
			Scope: Bucket + ":" + updataFileName,
			//设置Token过期时间
			Expires: 3600,
		}
		//生成一个上传token
		token := c.MakeUptoken(policy)

		//构建一个uploader
		zone := 0
		uploader := kodocli.NewUploader(zone, nil)

		var ret PutRet
		//调用PutFile方式上传，这里的key需要和上传指定的key一致

		res := uploader.PutFile(nil, &ret, token, updataFileName, singleFile, nil)
		fmt.Println(`成功上传文件"` + relativePathFileName + `"到"` + updataFileName + `"`)

		//打印出错信息
		if res != nil {
			return "io.Put failed:" + res.Error()
		}

	}

	fmt.Println("成功上传完文件到七牛存储服务器")

	return ""

}

func refreshFile(uploadHashFileData []string) string {

	if IsRefreshFile != "true" {
		fmt.Println("没有开启更新缓存，不更新。")
		return ""
	}

	//检查上次运行更新缓存有没有失败的文件
	failRefreshFile := ""
	if checkFileIsExist(FailRefreshPath) {
		var error string
		failRefreshFile, error = readFile(FailRefreshPath)
		checkError(error)
	}

	//准备更新缓存

	//读取上一次失败更新缓存文件和传进来要更新缓存的文件合并除重

	failRefreshFileSlice := strings.Split(failRefreshFile, ",")
	for singleKey, singleData := range failRefreshFileSlice {
		singleData = strings.Trim(singleData, `"`)
		failRefreshFileSlice[singleKey] = strings.Replace(singleData, DomainName+TargetPath, "", -1)
	}

	uniqData := uniq(failRefreshFileSlice, uploadHashFileData)

	if len(uniqData) > 100 {
		return "每天更新缓存文件数量不能超过100个"
	}

	urls := MergeUrls(uniqData)

	if urls == "" {
		fmt.Println("没有文件需要更新缓存,程序执行完毕")
		return ""
	}

	token := getToken([]byte("/v2/tune/refresh\n"))

ExecuteRefreshFile:

	client := &http.Client{}
	reqest, err :=
		http.NewRequest(
			"POST",
			"http://fusion.qiniuapi.com/v2/tune/refresh",
			strings.NewReader(`{"urls":[`+urls+`]}`),
		)

	if err != nil {
		return "http.NewRequest 出错内容:" + err.Error()
	}

	reqest.Header.Add("Authorization", "QBox "+token)
	reqest.Header.Add("Content-Type", "application/json")
	//接收返回的信息

	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return "client.Do 出错内容:" + err.Error()
	}

	bodyByte, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "ioutil.ReadAll 出错内容:" + err.Error()
	}

	responseData := map[string]interface{}{}
	err = json.Unmarshal([]byte(bodyByte), &responseData)
	if err != nil {
		return "json.Unmarshal 出错内容:" + err.Error()
	}

	if response.StatusCode != 200 {
		return "他返回的不是200，是" + strconv.Itoa(response.StatusCode) + ",错误内容是:" + string(bodyByte)
	}

	if responseData["code"].(float64) == 200 {
		fmt.Println("成功更新七牛文件缓存:")
		fmt.Println(urls)
		fmt.Println("你今天还有" + strconv.FormatFloat(responseData["urlSurplusDay"].(float64), 'f', -1, 64) + "个文件可以更新文件缓存")
		if checkFileIsExist(FailRefreshPath) {
			err = os.Remove(FailRefreshPath)
			if err != nil {
				return "os.Remove 出错内容:" + err.Error()
			}
		}

	} else {
		//如果更新缓存不成功，写入文件提醒下次运行再更新文件缓存
		writeFile(FailRefreshPath, urls)
		fmt.Println("错误内容:" + string(bodyByte))
		if RefreshFileRetryTimes <= 10 {
			fmt.Println("更新七牛文件缓存失败,60秒后会自动再提交七牛更新一下文件缓存，或者你重新执行copy2qiniu命令，将会重新更新七牛文件缓存，请稍后...")
		} else {
			return "更新缓存失败，已经执行缓存次数" + strconv.Itoa(RefreshFileRetryTimes) + "次，请联系七牛管理员查看原因。"
		}

		time.Sleep(60 * time.Second)
		RefreshFileRetryTimes += 1
		goto ExecuteRefreshFile
	}

	return ""

}

func getToken(data []byte) string {
	h := hmac.New(sha1.New, []byte(SecretKey))
	h.Write(data)

	sign := base64.URLEncoding.EncodeToString(h.Sum(nil))

	token := AccessKey + ":" + sign
	return token
}

func uniq(data1 []string, data2 []string) []string {
	//合并去重相同数据
	result := []string{}

	mergeData := append(data1, data2...)

	for _, i := range mergeData {

		if len(result) == 0 {
			if i != "" {
				result = append(result, i)
			}
		} else {
			for k, v := range result {
				if i == v {
					break
				}
				if k == len(result)-1 {
					if i != "" {
						result = append(result, i)
					}
				}
			}
		}
	}

	return result
}

func MergeUrls(data []string) string {
	urls := ""
	for _, singleUrl := range data {
		if urls != "" {
			urls += ","
		}
		urls = urls + `"` + DomainName + TargetPath + singleUrl + `"`
	}
	return urls
}

func checkError(ErrorData string) {
	if ErrorData != "" {
		fmt.Println(ErrorData)
		os.Exit(1)
	}
}

// 判断文件是否存在  存在返回 true 不存在返回false
func checkFileIsExist(filename string) bool {
	var exist bool

	_, err := os.Stat(filename)

	if !os.IsNotExist(err) {
		exist = true
	}

	return exist
}
