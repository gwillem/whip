package runners

import "github.com/nikolalohinski/gonja"

var tplParser = newTemplateParser()

func newTemplateParser() *gonja.Environment {
	cfg := gonja.NewConfig()
	cfg.StrictUndefined = true
	return gonja.NewEnvironment(cfg, gonja.DefaultLoader)
}
