#Copy2qiniu
可以一条命令语句，把所需要的文件自动复制到七牛储备上的工具

```
{
  "AccessKey":"qPioQd55AzEDOMqNYVccQAX8YqZTjlOamMNP7QOT",//七牛密匙
  "SecretKey":"jIZF04VCaSj-KKQSNHABqnXi0IcWWBNFKmNf_odw",//七牛密匙
  "Bucket":"hongbeibang-app",//对象资源名字
  "Website":"http://ochy6ops8.bkt.clouddn.com/",//刷新文件缓存要用的，必须斜号结尾
  "OriginDir":"./",//你需要上传文件的目录地址，必须斜号结尾
  "TargetDir":"app/",//你需要上传到七牛的那个文件夹，必须斜号结尾
  "IsRefreshFile":"true"//是否开启刷新缓存
}
```
