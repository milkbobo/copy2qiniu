# Copy2qiniu
可以一条命令语句，把所需要的文件自动复制到七牛储备上的工具。

# 运行流程
把所有需要上传的文件遍历出来上传（点前缀的隐藏文件忽略上传），然后配对文件Hash是否存在，如果文件不存在或者Hash不对，就上传，并且把文件存在而Hash不对的文件进行刷新缓存。因为目前七牛刷新文件缓存次数有限，这样比qiniu/qshell更省刷新次数。

# 配置方法
该程序必须安装golang语言，配置好后，在程序目录运行go install，就可以在需要上传的目录运行copy2qiniu命令使用了。

# 配置文件
必须在需要运行该命令的目录下，建立copy2qiniu.config.json文件，例子如下：

```
{
  "AccessKey":"***",//七牛密匙
  "SecretKey":"***",//七牛密匙
  "Bucket":"hongbeibang-app",//对象资源名字
  "DomainName":"http://app.hongbeibang.com/",//你要刷新文件缓存的域名，必须斜号结尾。PS:你刷新了那个域名，那个域名的文件就会起效果。
  "OriginPath":"./",//你需要上传文件的目录地址，必须斜号结尾
  "TargetPath":"app/",//你需要上传到七牛的那个文件夹，必须斜号结尾
  "AllowUploadFiles":"*", // 只上传的文件名，如果运行全部文件上传，填写 * , 可以填写文件名称。支持通配符方式填写，逗号(,)隔开。
  "IsRefreshFile":"true"//是否开启刷新缓存
}
```
