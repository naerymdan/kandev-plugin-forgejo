package forgejo

import (
	"context"
	"os"
	"strings"
	"testing"

	"kandev-plugin-forgejo/internal/sourcecontrol"

	"github.com/kandev/kandev/pkg/pluginsdk"
	"github.com/stretchr/testify/require"
)

// Live contract tests. They are skipped unless a disposable instance is
// supplied, so the default `go test ./...` stays hermetic:
//
//	KANDEV_FORGEJO_URL=http://localhost:3000 \
//	KANDEV_FORGEJO_TOKEN=... \
//	KANDEV_FORGEJO_OWNER=kandev KANDEV_FORGEJO_REPO=demo \
//	go test ./internal/forgejo -run TestLive -v
//
// Point them at Forgejo and at Gitea in turn: this plugin targets the REST v1
// surface both hosts share, and these tests are what prove that claim.
func liveConfig(t *testing.T) (baseURL, token, owner, repo string) {
	t.Helper()
	baseURL = strings.TrimSpace(os.Getenv("KANDEV_FORGEJO_URL"))
	token = strings.TrimSpace(os.Getenv("KANDEV_FORGEJO_TOKEN"))
	if baseURL == "" || token == "" {
		t.Skip("set KANDEV_FORGEJO_URL and KANDEV_FORGEJO_TOKEN to run live contract tests")
	}
	owner = strings.TrimSpace(os.Getenv("KANDEV_FORGEJO_OWNER"))
	repo = strings.TrimSpace(os.Getenv("KANDEV_FORGEJO_REPO"))
	require.NotEmpty(t, owner, "KANDEV_FORGEJO_OWNER is required")
	require.NotEmpty(t, repo, "KANDEV_FORGEJO_REPO is required")
	return baseURL, token, owner, repo
}

func liveAdapters(t *testing.T) (*Client, *Repositories, *Associations, *Reviews, *References, *fakeHost) {
	t.Helper()
	baseURL, token, _, _ := liveConfig(t)
	host := newFakeHost(map[string]any{"base_url": baseURL, "api_token": token})
	connection := NewConnection(func() pluginsdk.Host { return host })
	client, err := connection.Client(context.Background())
	require.NoError(t, err)
	repositories := NewRepositories(connection, func() pluginsdk.Host { return host }, "forgejo")
	associations := NewAssociations(func() pluginsdk.Host { return host })
	reviews := NewReviews(connection, repositories, associations, "forgejo")
	return client, repositories, associations, reviews, NewReferences(connection), host
}

func TestLiveInstanceSpeaksRESTv1(t *testing.T) {
	client, _, _, _, _, _ := liveAdapters(t)
	ctx := context.Background()

	version, err := client.Version(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, version)
	t.Logf("instance version: %s", version)

	user, err := client.CurrentUser(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, user.Login)
}

func TestLiveRepositoryDiscovery(t *testing.T) {
	client, repositories, _, _, _, _ := liveAdapters(t)
	_, _, owner, repo := liveConfig(t)
	ctx := context.Background()

	page, err := repositories.List(ctx, "workspace-1", "", sourcecontrol.RepositoryCursor{}, 10)
	require.NoError(t, err)
	require.NotEmpty(t, page.Repositories, "the token should see at least the test repository")

	var found *sourcecontrol.Repository
	for i := range page.Repositories {
		if page.Repositories[i].Name == repo && page.Repositories[i].OwnerOrProject == owner {
			found = &page.Repositories[i]
			break
		}
	}
	require.NotNil(t, found, "listing did not include %s/%s", owner, repo)
	require.Equal(t, client.Scope(), found.ConnectionScope)
	require.NotEmpty(t, found.CloneURL)
	require.NotEmpty(t, found.RepositoryID)

	// inspectURL is the ownership authority.
	inspected, err := repositories.Inspect(ctx, "workspace-1", client.Scope()+"/"+owner+"/"+repo)
	require.NoError(t, err)
	require.NotNil(t, inspected)
	require.Equal(t, found.RepositoryID, inspected.RepositoryID)

	foreign, err := repositories.Inspect(ctx, "workspace-1", "https://github.com/"+owner+"/"+repo)
	require.NoError(t, err)
	require.Nil(t, foreign, "a URL on another host must not be claimed")

	// Resolve by immutable id, then list its branches.
	resolved, err := repositories.Resolve(ctx, "workspace-1", sourcecontrol.RepositoryIdentity{
		ConnectionScope: found.ConnectionScope,
		RepositoryID:    found.RepositoryID,
	})
	require.NoError(t, err)
	require.Equal(t, repo, resolved.Name)

	branches, err := repositories.ListBranches(ctx, "workspace-1", resolved)
	require.NoError(t, err)
	require.NotEmpty(t, branches)
	var sawDefault bool
	for _, branch := range branches {
		if branch.IsDefault {
			sawDefault = true
		}
	}
	require.True(t, sawDefault, "the default branch must be flagged")
}

func TestLiveReviewSnapshot(t *testing.T) {
	client, repositories, associations, reviews, _, host := liveAdapters(t)
	_, _, owner, repo := liveConfig(t)
	ctx := context.Background()

	inspected, err := repositories.Inspect(ctx, "workspace-1", client.Scope()+"/"+owner+"/"+repo)
	require.NoError(t, err)
	require.NotNil(t, inspected)

	host.tasks["task-1"] = &pluginsdk.Task{ID: "task-1", WorkspaceID: "workspace-1"}
	identity := sourcecontrol.ChangeRequestIdentity{
		ConnectionScope: inspected.ConnectionScope,
		RepositoryID:    inspected.RepositoryID,
		Number:          1,
	}
	require.NoError(t, associations.Link(ctx, "task-1", identity))

	snapshots, err := reviews.ForTask(ctx, "workspace-1", "task-1")
	require.NoError(t, err)
	require.Len(t, snapshots, 1, "pull request #1 must exist in the test repository")

	snapshot := snapshots[0]
	require.Contains(t, []string{"open", "draft", "merged", "closed"}, snapshot.State)
	require.NotEmpty(t, snapshot.Title)
	require.NotEmpty(t, snapshot.URL)
	require.NotNil(t, snapshot.TaskStatus)
	require.Equal(t, int64(1), snapshot.TaskStatus.Number)
	require.Contains(t, []string{"success", "failure", "pending", "neutral"}, snapshot.TaskStatus.PipelineState)
	t.Logf("state=%s pipeline=%s checks=%d review=%+v",
		snapshot.State, snapshot.TaskStatus.PipelineState, len(snapshot.TaskStatus.Checks), snapshot.TaskStatus.Review)

	// The workspace map must agree with the per-task snapshot on reviewKey.
	workspace, err := reviews.Associations(ctx, "workspace-1")
	require.NoError(t, err)
	require.Len(t, workspace, 1)
	require.Equal(t, snapshot.ReviewKey, workspace[0].ReviewKey)

	require.NoError(t, associations.Unlink(ctx, "task-1", identity))
	after, err := reviews.Associations(ctx, "workspace-1")
	require.NoError(t, err)
	require.Empty(t, after)
}

func TestLiveReferenceResolutionAndAuthorization(t *testing.T) {
	client, repositories, _, _, references, host := liveAdapters(t)
	_, _, owner, repo := liveConfig(t)
	ctx := context.Background()

	connection := NewConnection(func() pluginsdk.Host { return host })
	changeRequests := NewChangeRequests(connection, repositories)

	resolved, err := changeRequests.ResolveReference(ctx, "workspace-1", owner+"/"+repo+"#1")
	require.NoError(t, err)
	require.Equal(t, int64(1), resolved.Identity.Number)
	require.Equal(t, client.Scope(), resolved.Identity.ConnectionScope)

	// The same pull request addressed by URL must resolve identically.
	byURL, err := changeRequests.ResolveReference(ctx, "workspace-1",
		client.Scope()+"/"+owner+"/"+repo+"/pulls/1")
	require.NoError(t, err)
	require.Equal(t, resolved.Identity, byURL.Identity)

	candidates, err := references.Search(ctx, "workspace-1", "", 10)
	require.NoError(t, err)
	require.NotEmpty(t, candidates, "the composer picker should find the test pull request")

	allowed, err := references.Authorize(ctx, "workspace-1", "submission",
		map[string]any{"id": candidates[0].ProviderLocalID})
	require.NoError(t, err)
	require.True(t, allowed)

	// A reference to something that does not exist must fail closed.
	denied, err := references.Authorize(ctx, "workspace-1", "submission",
		map[string]any{"id": "999999:999999"})
	require.NoError(t, err)
	require.False(t, denied)
}

func TestLiveCreatePullRequest(t *testing.T) {
	branch := strings.TrimSpace(os.Getenv("KANDEV_FORGEJO_HEAD_BRANCH"))
	if branch == "" {
		t.Skip("set KANDEV_FORGEJO_HEAD_BRANCH to exercise pull-request creation")
	}
	client, repositories, _, _, _, host := liveAdapters(t)
	_, _, owner, repo := liveConfig(t)
	ctx := context.Background()

	inspected, err := repositories.Inspect(ctx, "workspace-1", client.Scope()+"/"+owner+"/"+repo)
	require.NoError(t, err)
	require.NotNil(t, inspected)

	connection := NewConnection(func() pluginsdk.Host { return host })
	created, err := NewChangeRequests(connection, repositories).Create(ctx, *inspected, branch,
		sourcecontrol.CreateChangeRequestInput{
			Title:       "Kandev plugin live test",
			Description: "Opened by the kandev-plugin-forgejo live contract test.",
			Draft:       true,
		})
	require.NoError(t, err)
	require.Positive(t, created.Identity.Number)
	require.NotEmpty(t, created.URL)
	t.Logf("created %s", created.URL)

	// A draft request is marked with the portable WIP title prefix.
	pull, err := client.PullRequest(ctx, owner, repo, created.Identity.Number)
	require.NoError(t, err)
	require.Equal(t, "draft", pullRequestState(pull))
}
