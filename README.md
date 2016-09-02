#Copy2qiniu
可以一条命令语句，把所需要的文件自动复制到七牛储备上的工具

#配置方法
该程序必须安装golang语言，配置好后，在程序目录运行go install，就可以在需要上传的目录运行copy2qiniu命令使用了。

#配置文件
必须在需要运行该命令的目录下，建立copy2qiniu.config.json文件，例子如下：

```
{
  "AccessKey":"***",//七牛密匙
  "SecretKey":"***",//七牛密匙
  "Bucket":"hongbeibang-app",//对象资源名字
  "DomainName":"http://app.hongbeibang.com/",//你要刷新文件缓存的域名，必须斜号结尾。PS:你刷新了那个域名，那个域名的文件就会起效果。
  "OriginPath":"./",//你需要上传文件的目录地址，必须斜号结尾
  "TargetPath":"app/",//你需要上传到七牛的那个文件夹，必须斜号结尾
  "IsRefreshFile":"true"//是否开启刷新缓存
}
```
