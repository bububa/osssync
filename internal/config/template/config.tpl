{{- range $v := .Settings}}
[[Settings]]
Name = "{{$v.Name}}"
Local = "{{$v.Local}}"
Endpoint = "{{$v.Endpoint}}"
AccessKeyID = "{{$v.AccessKeyID}}"
AccessKeySecret = "{{$v.AccessKeySecret}}"
Bucket = "{{$v.Bucket}}"
Prefix = "{{$v.Prefix}}"
IgnoreHiddenFiles = {{$v.IgnoreHiddenFiles}}
Delete = {{$v.Delete}}
{{end}}
