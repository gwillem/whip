package whip

/*

Need to do custom YAML parsing, because Ansible syntax uses dynamic dict
keys as plugin names

*/

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadPlaybook(path string) Playbook {
	yamlData, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	rawPb := RawPlaybook{}
	pb := Playbook{}

	if e := yaml.Unmarshal(yamlData, &rawPb); e != nil {
		panic(e)
	}

	for _, rawPlay := range rawPb {
		play := Play{}

		play.Hosts = parseHosts(rawPlay.Hosts)
		// play.Targets = rawPlay.Targets
		for _, rawTask := range rawPlay.RawTasks {
			task := Task{Args: TaskArgs{}}
			for k, v := range rawTask {

				switch k {
				case "name":
					task.Name = v.(string)
					continue
				case "with_items":
					task.Items = v.([]any)
					continue
				}

				task.Type = k
				if val, ok := v.(string); ok {
					task.Args = TaskArgs{}
					for k, v := range parseArgString(val) {
						task.Args[k] = v
					}
				}

				if val, ok := v.(AnyMap); ok {
					for k, v := range val {
						task.Args[k] = v.(string)
					}
				}
			}
			play.Tasks = append(play.Tasks, task)
		}
		pb = append(pb, play)
	}
	return pb
}

func parseHosts(anyHosts any) []Host {
	ret := []Host{}
	switch hosts := anyHosts.(type) {
	case string:
		for _, t := range strings.Split(hosts, ",") {
			ret = append(ret, Host(strings.TrimSpace(t)))
		}
	case []string:
		for _, t := range hosts {
			fmt.Println("found string", t)
			ret = append(ret, Host(t))
		}
	case []any:
		for _, h := range hosts {
			switch t := h.(type) {
			case string:
				ret = append(ret, Host(t))
			default:
				panic(fmt.Sprintf("unknown host type %T", t))
			}
		}
	default:
		panic(fmt.Sprintf("unknown hosts field %T", hosts))
	}
	return ret
}

func parseArgString(arg string) map[string]string {
	kv := map[string]string{}

	baseArgs := []string{}
	for _, t := range strings.Split(arg, " ") {
		if strings.Contains(t, "=") {
			opt := strings.SplitN(t, "=", 2)

			kv[opt[0]] = unquote(opt[1])
		} else {
			baseArgs = append(baseArgs, t)
		}
	}

	kv["_args"] = strings.Join(baseArgs, " ")
	return kv
}

func unquote(s string) string {
	if n, e := strconv.Unquote(s); e == nil {
		return n
	}
	return s
}
