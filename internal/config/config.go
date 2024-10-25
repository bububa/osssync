package config

type Config struct {
	Settings []Setting `required:"true"`
}

type Setting struct {
	Local string `required:"true"`
	Credential
	IgnoreHiddenFiles bool
	Delete            bool
}

type Credential struct {
	Endpoint        string `required:"true"`
	AccessKeyID     string `required:"true"`
	AccessKeySecret string `required:"true"`
	Bucket          string `required:"true"`
	Prefix          string
}
