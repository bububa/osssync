package config

import "fmt"

var EmptySetting Setting

type Config struct {
	Settings []Setting
}

type Setting struct {
	Name  string `required:"true"`
	Local string `required:"true"`
	Credential
	IgnoreHiddenFiles bool
	Delete            bool
}

func (s Setting) Key() string {
	return fmt.Sprintf("%s | %s", s.Local, s.BucketKey())
}

func (s Setting) BucketKey() string {
	return fmt.Sprintf("%s/%s", s.Bucket, s.Prefix)
}

func (s Setting) DisplayName() string {
	if s.Name == "" {
		return s.Key()
	}
	return s.Name
}

type Credential struct {
	Endpoint        string `required:"true"`
	AccessKeyID     string `required:"true"`
	AccessKeySecret string `required:"true"`
	Bucket          string `required:"true"`
	Prefix          string `required:"true"`
}
