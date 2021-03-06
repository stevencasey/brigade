package webhook

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/github"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Azure/brigade/pkg/brigade"
	"github.com/Azure/brigade/pkg/storage"
)

type testStore struct {
	proj   *brigade.Project
	builds []*brigade.Build
	err    error
	storage.Store
}

func (s *testStore) GetProject(name string) (*brigade.Project, error) {
	return s.proj, s.err
}

func (s *testStore) CreateBuild(build *brigade.Build) error {
	s.builds = append(s.builds, build)
	return s.err
}

func newTestStore() *testStore {
	return &testStore{
		proj: &brigade.Project{
			Name:         "baxterthehacker/public-repo",
			SharedSecret: "asdf",
		},
	}
}

func newTestGithubHandler(store storage.Store, t *testing.T) *githubHook {
	s := NewGithubHook(store, false)
	s.getFile = func(commit, path string, proj *brigade.Project) ([]byte, error) {
		return []byte(""), nil
	}
	s.createStatus = func(commit string, proj *brigade.Project, status *github.RepoStatus) error {
		return nil
	}
	return s
}

func TestGithubHandler(t *testing.T) {
	store := newTestStore()
	s := newTestGithubHandler(store, t)
	s.createStatus = func(commit string, proj *brigade.Project, status *github.RepoStatus) error {
		if status.GetState() != "pending" {
			t.Error("RepoStatus.State is not correct")
		}
		if status.GetDescription() != "Building" {
			t.Error("RepoStatus.Building is not correct")
		}
		if commit != "0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c" {
			t.Error("commit is not correct")
		}
		return nil
	}

	payload, err := ioutil.ReadFile("testdata/github-push-payload.json")
	if err != nil {
		t.Fatalf("failed to read testdata: %s", err)
	}

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", "", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("failed to create request: %s", err)
	}
	r.Header.Add("X-GitHub-Event", "push")
	r.Header.Add("X-Hub-Signature", SHA1HMAC([]byte("asdf"), payload))

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = r

	s.Handle(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected error: %d\n%s", w.Code, w.Body.String())
	}
	if len(store.builds) != 1 {
		t.Fatal("expected a build created")
	}
	if store.builds[0].Type != "push" {
		t.Error("Build.Type is not correct")
	}
	if store.builds[0].Provider != "github" {
		t.Error("Build.Provider is not correct")
	}
	if store.builds[0].Commit != "0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c" {
		t.Error("Build.Commit is not correct")
	}
}

func TestGithubHandler_ping(t *testing.T) {
	store := newTestStore()
	s := newTestGithubHandler(store, t)

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		t.Fatalf("failed to create request: %s", err)
	}
	r.Header.Add("X-GitHub-Event", "ping")

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = r

	s.Handle(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected error: %d\n%s", w.Code, w.Body.String())
	}
}

func TestGithubHandler_badevent(t *testing.T) {
	store := newTestStore()
	s := newTestGithubHandler(store, t)

	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		t.Fatalf("failed to create request: %s", err)
	}
	r.Header.Add("X-GitHub-Event", "funzone")

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = r

	s.Handle(ctx)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid X-GitHub-Event Header") {
		t.Fatalf("unexpected body: %d\n%s", w.Code, w.Body.String())
	}
}

func TestTruncAt(t *testing.T) {
	if "foo" != truncAt("foo", 100) {
		t.Fatal("modified string that was fine.")
	}

	if got := truncAt("foobar", 6); got != "foobar" {
		t.Errorf("Unexpected truncation of foobar: %s", got)
	}

	if got := truncAt("foobar1", 6); got != "foo..." {
		t.Errorf("Unexpected truncation of foobar1: %s", got)
	}
}
