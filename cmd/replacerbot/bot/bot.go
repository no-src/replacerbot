package bot

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/no-src/log"
	"github.com/no-src/nsgo/browser"
	"github.com/no-src/nsgo/httputil"
	"github.com/no-src/nsgo/jsonutil"
	"github.com/no-src/replacerbot/cmd/replacer-starter/starter"
)

var (
	RepoUrl         string
	Username        string
	Password        string
	OriginBranch    string
	SavePath        string
	Tag             string
	CommitMessage   string
	ReplacerFile    string
	ReplacerUrl     string
	ReplacerConf    string
	ReplacerConfUrl string
	GitlabGroupId   int
	GitlabToken     string
	Revert          bool
)

func InitFlags() {
	flag.StringVar(&RepoUrl, "repo_url", "", "git repository url, required")
	flag.StringVar(&Username, "username", "", "gitlab username, required")
	flag.StringVar(&Password, "password", "", "gitlab password, required")
	flag.StringVar(&OriginBranch, "branch", "", "branch name, required")
	flag.StringVar(&Tag, "tag", "", "tag name")
	flag.StringVar(&SavePath, "work_dir", "./repo", "workspace directory, optional")
	flag.StringVar(&CommitMessage, "commit_message", "", "git commit message")
	flag.StringVar(&ReplacerFile, "replacer_file", "", "local replacer file path")
	flag.StringVar(&ReplacerUrl, "replacer_url", "", "remote replacer file url")
	flag.StringVar(&ReplacerConf, "replacer_conf", "", "local replacer config file path")
	flag.StringVar(&ReplacerConfUrl, "replacer_conf_url", "", "remote replacer config file url")
	flag.IntVar(&GitlabGroupId, "gitlab_group_id", 0, "GroupId in gitlab, see: /api/v4/groups")
	flag.StringVar(&GitlabToken, "gitlab_token", "", "token in gitlab, see: /profile/personal_access_tokens")
	flag.BoolVar(&Revert, "revert", false, "revert the replace operation")
	flag.Parse()
}

func RunWithFlags() error {
	return Run(RepoUrl, Username, Password, OriginBranch, Tag, SavePath, CommitMessage, ReplacerFile, ReplacerUrl, ReplacerConf, ReplacerConfUrl, GitlabGroupId, GitlabToken, Revert)
}

func Run(repoUrl, username, password, originBranch, tag, savePath, commitMessage, replacerFile, replacerUrl, replacerConf, replacerConfUrl string, gitlabGroupId int, gitlabToken string, revert bool) error {
	newBranch := originBranch + "-dev-replacer-" + time.Now().Format("20060102150405")
	if revert {
		newBranch = "revert-" + newBranch
	}
	repoRoot := savePath
	auth := &githttp.BasicAuth{
		Username: username,
		Password: password,
	}

	repoUrlObj, err := url.Parse(repoUrl)
	if err != nil {
		log.Error(err, "parse git repo url error: %s", repoUrl)
		return err
	}

	repoFullName := strings.Trim(strings.TrimSuffix(repoUrlObj.Path, ".git"), "/")
	repoSchema := repoUrlObj.Scheme
	repoHost := repoUrlObj.Host

	_, err = os.Stat(savePath)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(savePath, os.ModePerm); err != nil {
			log.Error(err, "create dir error=> %s", savePath)
			return err
		}
	}
	if err != nil {
		log.Error(err, "init dir error=> %s", savePath)
		return err
	}

	repoRoot, err = os.MkdirTemp(savePath, strings.ReplaceAll(repoFullName, "/", "-")+"-")
	if err != nil {
		log.Error(err, "init repo dir error")
		return err
	}

	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		log.Error(err, "get repo abs path error => %s", repoRoot)
		return err
	}

	repo, err := git.PlainClone(repoRoot, false, &git.CloneOptions{
		URL:           repoUrl,
		Progress:      os.Stdout,
		SingleBranch:  true,
		ReferenceName: plumbing.ReferenceName(originBranch),
		Auth:          auth,
	})

	if err != nil {
		log.Error(err, "clone repo error => %s", repoUrl)
		return err
	}

	tree, err := repo.Worktree()
	if err != nil {
		log.Error(err, "get repo worktree error => %s", repoUrl)
		return err
	}

	err = tree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(newBranch),
		Create: true,
	})
	if err != nil {
		log.Error(err, "checkout error %s => %s", repoUrl, newBranch)
		return err
	}

	err = starter.Run(repoRoot, tag, replacerConf, replacerConfUrl, replacerFile, replacerUrl, revert)
	if err != nil {
		log.Error(err, "run starter error")
		return err
	}

	if revert {
		commitMessage = fmt.Sprintf("chore(revert replace %s): %s", tag, commitMessage)
	} else {
		commitMessage = fmt.Sprintf("chore(replace %s): %s", tag, commitMessage)
	}
	hash, err := tree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "replacerbot",
			Email: username,
			When:  time.Now(),
		},
		All: true,
	})
	if err != nil {
		log.Error(err, "git commit error %s => %s", repoUrl, newBranch)
		return err
	}

	log.Info("git commit success: %s", hash.String())

	err = repo.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil {
		log.Error(err, "git push error %s => %s", repoUrl, newBranch)
		return err
	}

	log.Info("remote repository url: %s", repoUrl)
	log.Info("local repository path: %s", repoRoot)
	log.Info("new branch: %s", newBranch)

	projectId, err := getProjectId(repoSchema, repoHost, gitlabGroupId, repoFullName, gitlabToken)
	if err != nil {
		log.Error(err, "get repository project id error %s => %s", repoFullName)
		return err
	}
	log.Info("current repository project id: %d", projectId)
	mrUrl := openCreateMR(repoSchema, repoHost, repoFullName, newBranch, originBranch, projectId)
	log.Info("if the browser does not open automatically, manually visit the following link to create a merge request: \n%s", mrUrl)
	return nil
}

func openCreateMR(schema, host, fullName, sourceBranch, targetBranch string, projectId int) string {
	url := "%s://%s/%s/merge_requests/new?utf8=✓&merge_request[source_project_id]=%d&merge_request[source_branch]=%s&merge_request[target_project_id]=%d&merge_request[target_branch]=%s"
	url = fmt.Sprintf(url, schema, host, fullName, projectId, sourceBranch, projectId, targetBranch)
	url = strings.ReplaceAll(url, "[", "%5B")
	url = strings.ReplaceAll(url, "]", "%5D")
	browser.OpenBrowser(url)
	return url
}

func getProjectId(schema, host string, groupId int, projectFullName string, token string) (int, error) {
	client, err := httputil.NewHttpClient(true, "", false)
	if err != nil {
		return 0, err
	}
	apiUrl := fmt.Sprintf("%s://%s/api/v4/groups/%d/projects", schema, host, groupId)
	resp, err := client.HttpGetWithCookie(apiUrl, http.Header{"PRIVATE-TOKEN": []string{token}})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var projects []project
	err = jsonutil.Unmarshal(data, &projects)
	if err != nil {
		return 0, err
	}
	id := 0
	for _, p := range projects {
		if strings.ToLower(p.PathWithNamespace) == strings.ToLower(projectFullName) {
			id = p.Id
			break
		}
	}
	if id <= 0 {
		err = errors.New("get repository project id error")
	}
	return id, err
}

type project struct {
	Id                int    `json:"id"`
	SSHUrlToRepo      string `json:"ssh_url_to_repo"`
	HttpUrlToRepo     string `json:"http_url_to_repo"`
	WebUrl            string `json:"web_url"`
	PathWithNamespace string `json:"path_with_namespace"`
}
