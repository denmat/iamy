package iamy

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/99designs/iamy/Godeps/_workspace/src/gopkg.in/yaml.v2"
)

var Yaml = YamlLoadDumper{
	pathTemplate: "{{.Account}}/{{.Resource.Type}}{{.Resource.Path}}{{.Resource.Name}}.yaml",
	pathRegex:    regexp.MustCompile(`^(?P<account>.+)/(?P<entity>(user|group|policy|role))(?P<path>.*/)(?P<name>.+)\.yaml$`),
}

type YamlLoadDumper struct {
	pathTemplate string
	pathRegex    *regexp.Regexp
	Dir          string
}

func (a *YamlLoadDumper) getFilesRecursively() ([]string, error) {
	paths := []string{}
	err := filepath.Walk(a.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		path, err = filepath.Rel(a.Dir, path)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})

	return paths, err
}

func namedMatch(r *regexp.Regexp, s string) (bool, map[string]string) {
	match := r.FindStringSubmatch(s)
	if len(match) == 0 {
		return false, nil
	}

	result := make(map[string]string)
	for i, name := range r.SubexpNames() {
		result[name] = match[i]
	}
	return true, result
}

func (a *YamlLoadDumper) Load() ([]AccountData, error) {
	log.Println("Loading YAML IAM data from", a.Dir)
	accounts := map[string]*AccountData{}

	allFiles, err := a.getFilesRecursively()
	if err != nil {
		return nil, err
	}

	for _, fp := range allFiles {

		if matched, result := namedMatch(a.pathRegex, fp); matched {
			log.Println("Loading", fp)

			accountid := result["account"]
			entity := result["entity"]
			path := result["path"]
			name := result["name"]

			if _, ok := accounts[accountid]; !ok {
				accounts[accountid] = NewAccountData(accountid)
			}

			switch entity {
			case "user":
				u := User{}
				err := a.unmarshalYamlFile(fp, &u)
				if err != nil {
					return nil, err
				}
				u.Name = name
				u.Path = path
				accounts[accountid].addUser(u)
			case "group":
				g := Group{}
				err := a.unmarshalYamlFile(fp, &g)
				if err != nil {
					return nil, err
				}
				g.Name = name
				g.Path = path
				accounts[accountid].addGroup(g)
			case "role":
				r := Role{}
				err := a.unmarshalYamlFile(fp, &r)
				if err != nil {
					return nil, err
				}
				r.Name = name
				r.Path = path
				accounts[accountid].addRole(r)
			case "policy":
				p := Policy{}
				err := a.unmarshalYamlFile(fp, &p)
				if err != nil {
					return nil, err
				}
				p.Name = name
				p.Path = path
				accounts[accountid].addPolicy(p)
			default:
				panic("Unexpected entity")
			}
		} else {
			log.Println("Skipping", fp)
		}
	}

	accts := []AccountData{}
	for _, a := range accounts {
		accts = append(accts, *a)
	}

	return accts, nil
}

func (f *YamlLoadDumper) Dump(accountData *AccountData, canDelete bool) error {
	destDir := filepath.Join(f.Dir, accountData.Account.String())
	log.Println("Dumping YAML IAM data to", f.Dir)

	if canDelete {
		if err := os.RemoveAll(destDir); err != nil {
			return err
		}
	}

	for _, u := range accountData.Users {
		if err := f.writeUser(accountData.Account, u); err != nil {
			return err
		}
	}

	for _, policy := range accountData.Policies {
		if err := f.writePolicy(accountData.Account, policy); err != nil {
			return err
		}
	}

	for _, group := range accountData.Groups {
		if err := f.writeGroup(accountData.Account, group); err != nil {
			return err
		}
	}

	for _, role := range accountData.Roles {
		if err := f.writeRole(accountData.Account, role); err != nil {
			return err
		}
	}

	return nil
}

func (f *YamlLoadDumper) writeUser(a *Account, u User) error {
	path, err := renderPath(f.pathTemplate, map[string]interface{}{
		"Account":  a,
		"Resource": u,
	})
	if err != nil {
		return err
	}
	return writeYamlFile(filepath.Join(f.Dir, path), u)
}

func (f *YamlLoadDumper) unmarshalYamlFile(relativePath string, entity interface{}) error {
	path := filepath.Join(f.Dir, relativePath)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(data, entity)
	if err != nil {
		return err
	}

	return nil
}

func (f *YamlLoadDumper) writeGroup(a *Account, g Group) error {
	path, err := renderPath(f.pathTemplate, map[string]interface{}{
		"Account":  a,
		"Resource": g,
	})
	if err != nil {
		return err
	}
	return writeYamlFile(filepath.Join(f.Dir, path), g)
}

func (f *YamlLoadDumper) writePolicy(a *Account, p Policy) error {
	path, err := renderPath(f.pathTemplate, map[string]interface{}{
		"Account":  a,
		"Resource": p,
	})
	if err != nil {
		return err
	}
	return writeYamlFile(filepath.Join(f.Dir, path), p)
}

func (f *YamlLoadDumper) writeRole(a *Account, r Role) error {
	path, err := renderPath(f.pathTemplate, map[string]interface{}{
		"Account":  a,
		"Resource": r,
	})
	if err != nil {
		return err
	}
	return writeYamlFile(filepath.Join(f.Dir, path), r)
}

func renderPath(tpl string, context map[string]interface{}) (string, error) {
	t, err := template.New("tpl").Parse(tpl)
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	if err = t.Execute(buf, context); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func writeYamlFile(path string, thing interface{}) error {
	b, err := yaml.Marshal(thing)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)

	if err = os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	if err = ioutil.WriteFile(path, b, 0666); err != nil {
		return err
	}

	return nil
}
