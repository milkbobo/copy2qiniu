package main

import (
	"./qetag"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"qiniupkg.com/api.v7/conf"
	"qiniupkg.com/api.v7/kodo"
	"qiniupkg.com/api.v7/kodocli"
	"strconv"
	"strings"
)

var (
	AccessKey          string
	SecretKey          string
	Bucket             string
	Website            string
	OriginDir          string
	OriginAbsolutePath string
	TargetDir          string
	IsRefreshFile      string
)

type PutRet struct {
	Hash string `json:"hash"`
	Key  string `json:"key"`
}

func main() {

	//获取配置文件
	config, error := readFile("config.json")
	checkError(error)

	//把json配置文件解析
	error = getConfig(config)
	checkError(error)

	//或者文件夹下所有文件，除了隐藏文件意外
	DirInfo, error := ReadDir(OriginAbsolutePath)
	checkError(error)

	//获得需要上传的文件
	uploadFileData, uploadHashFileData, error := getFileInfo(DirInfo)
	checkError(error)

	//上传文件
	error = updataFile(uploadFileData)
	checkError(error)

	//刷新七牛文件的缓存
	error = refreshFile(uploadHashFileData)
	checkError(error)

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
		return "filepath.Abs 出错内容:" + err.Error()
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

	OriginDir, ok = configData["OriginDir"]
	if OriginDir == "" || ok == false {
		return "OriginDir配置参数不存在或者不能为空"
	}

	TargetDir, ok = configData["TargetDir"]
	if TargetDir == "" || ok == false {
		return "TargetDir配置参数不存在或者不能为空"
	}

	IsRefreshFile, ok = configData["IsRefreshFile"]
	if IsRefreshFile == "" || ok == false {
		return "IsRefreshFile配置参数不存在或者不能为空"
	}

	Website, ok = configData["Website"]
	if Website == "" || ok == false {
		return "Website配置参数不存在或者不能为空"
	}

	conf.ACCESS_KEY = AccessKey
	conf.SECRET_KEY = SecretKey

	//获取绝对路径
	OriginAbsolutePath, err = filepath.Abs(OriginDir)
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

func ReadDir(path string) ([]string, string) {
	result := []string{}
	//获取文件夹所有的文件列表
	tempResult, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, "ioutil.ReadAll 出错内容:" + err.Error()
	}
	for _, singleFileInfo := range tempResult {

		//如果是隐藏文件，就忽略
		if singleFileInfo.Name()[0:1] == "." {
			continue
		}

		name := path + "/" + singleFileInfo.Name()

		if singleFileInfo.IsDir() {
			//发现dir
			result2, err := ReadDir(name)
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

func getFileInfo(fileName []string) ([]string, []string, string) {

	uploadFileData := []string{}
	uploadHashFileData := []string{}

	//new一个Bucket管理对象
	c := kodo.New(0, nil)
	p := c.Bucket(Bucket)

	for _, singleFile := range fileName {

		//调用Stat方法获取文件的信息
		relativePathFileName := strings.Replace(singleFile, OriginAbsolutePath, "", -1)[1:]
		fileName := TargetDir + relativePathFileName
		entry, err := p.Stat(nil, fileName)
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
		hashData, err := qetag.GetEtag(singleFile)
		if err != nil {
			return nil, nil, "qetag.GetEtag 出错内容:" + err.Error()
		}

		//如果hash值不同，也添加到上传队列中
		if entry.Hash != hashData {
			uploadHashFileData = append(uploadHashFileData, relativePathFileName)
			uploadFileData = append(uploadFileData, singleFile)
			continue
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

		updataFileName := TargetDir + relativePathFileName

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

		//打印出错信息
		if res != nil {
			return "io.Put failed:" + res.Error()
		}

	}

	fmt.Println("成功上传完文件到七牛")

	return ""

}

func refreshFile(uploadHashFileData []string) string {

	if IsRefreshFile != "true" {
		fmt.Println("没有开启刷新缓存,程序执行完毕")
		return ""
	}

	if len(uploadHashFileData) == 0 {
		fmt.Println("没有文件需要文件刷新缓存,程序执行完毕")
		return ""
	}

	if len(uploadHashFileData) > 100 {
		return "每天更新缓存文件数量不能超过100个"
	}

	//准备更新缓存的资料

	token := getToken([]byte("/v2/tune/refresh\n"))

	client := &http.Client{}

	urls := ""

	for _, singleUrl := range uploadHashFileData {
		if urls != "" {
			urls += ","
		}
		urls = urls + `"` + Website + singleUrl + `"`
	}

	reqest, err :=
		http.NewRequest(
			"POST",
			"http://fusion.qiniuapi.com/v2/tune/refresh", strings.NewReader(`{"urls":[`+urls+`]}`),
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
		return "filepath.Abs 出错内容:" + err.Error()
	}

	if response.StatusCode != 200 {
		return "他返回的不是200，是" + strconv.Itoa(response.StatusCode) + ",错误内容是:" + string(bodyByte)
	}

	if responseData["code"].(float64) == 200 {
		fmt.Println("成功刷新七牛文件缓存,程序执行完毕")
	} else {

		return string(bodyByte)
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

func checkError(ErrorData string) {
	if ErrorData != "" {
		fmt.Println(ErrorData)
		os.Exit(1)
	}
}
