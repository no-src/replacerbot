package starter

import (
	"flag"
	"os/exec"

	"github.com/no-src/log"
	"github.com/no-src/nsgo/httputil"
)

var (
	Root            string
	Tag             string
	ReplacerConf    string
	ReplacerConfUrl string
	ReplacerFile    string
	ReplacerUrl     string
	Revert          bool
)

func InitFlags() {
	flag.StringVar(&Tag, "tag", "", "the tag name")
	flag.StringVar(&Root, "root", "./", "the root workspace")
	flag.StringVar(&ReplacerFile, "replacer_file", "", "the local path of the replacer")
	flag.StringVar(&ReplacerUrl, "replacer_url", "", "the remote url of the replacer")
	flag.StringVar(&ReplacerConf, "replacer_conf", "", "the local config file of the replacer")
	flag.StringVar(&ReplacerConfUrl, "replacer_conf_url", "", "the remote config file of the replacer")
	flag.BoolVar(&Revert, "revert", false, "revert the replace operations")
	flag.Parse()
}

func RunWithFlags() error {
	return Run(Root, Tag, ReplacerConf, ReplacerConfUrl, ReplacerFile, ReplacerUrl, Revert)
}

func Run(root, tag, replacerConf, replacerConfUrl, replacerFile, replacerUrl string, revert bool) error {
	if len(replacerUrl) > 0 {
		client, err := httputil.NewHttpClient(true, "", false)
		if err != nil {
			log.Error(err, "init http client error")
			return err
		}
		if err = client.Download(replacerFile, replacerUrl, false); err != nil {
			log.Error(err, "download replacer error, url => %s", replacerUrl)
			return err
		}
	}
	args := []string{
		"-root=" + root,
		"-tag=" + tag,
		"-conf=" + replacerConf,
		"-conf_url=" + replacerConfUrl}
	if revert {
		args = append(args, "-revert")
	}
	cmd := exec.Command(replacerFile, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Error(err, "run replacer error")
	} else {
		log.Info("run replacer success")
	}
	log.Info("replacer running result:")
	log.Info(string(out))
	return err
}
