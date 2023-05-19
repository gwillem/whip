package runners

import (
	"fmt"
	"testing"

	"github.com/nikolalohinski/gonja"
)

func TestGonja(t *testing.T) {

	vars := map[string]any{
		"item":    "banaan",
		"changed": true,
	}

	cfg := gonja.NewConfig()
	cfg.StrictUndefined = true

	env := gonja.NewEnvironment(cfg, gonja.DefaultLoader)
	tpl, err := env.FromString("Hello {{ item | capitalize }}! This playbook was {{changed}} {{kjsdfhksd}}")

	// tpl, err := gonja.FromString("Hello {{ item | capitalize }}! This playbook was {{unknopwn}}")
	if err != nil {
		panic(err)
	}
	out, err := tpl.Execute(vars)
	if err != nil {
		panic(err)
	}
	fmt.Println(out) // Prints: Hello Bob!
}
