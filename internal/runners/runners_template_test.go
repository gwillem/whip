package runners

// func TestGonja(t *testing.T) {
// 	vars := map[string]any{
// 		"item":    "banaan",
// 		"changed": true,
// 	}

// 	cfg := gonja.NewConfig()
// 	cfg.StrictUndefined = true

// 	env := gonja.NewEnvironment(cfg, gonja.DefaultLoader)
// 	tpl, err := env.FromString("Hello {{ item | capitalize }}! This playbook was {{changed}} {{kjsdfhksd}}")
// 	// tpl, err := gonja.FromString("Hello {{ item | capitalize }}! This playbook was {{unknopwn}}")
// 	if err != nil {
// 		panic(err)
// 	}
// 	want := fmt.Sprintf("Hello %s! This playbook was %t ", "banaan", true)
// 	got, err := tpl.Execute(vars)
// 	require.NoError(t, err)
// 	require.Equal(t, want, got)
// }
