package login

import (
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/Bplotka/oidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testProvider = "https://example.org"
)

func TestK8sCache_Token(t *testing.T) {
	loginCfg := Config{
		ClientID:     "ID1",
		ClientSecret: "secret1",
		NonceCheck:   true,
		Scopes: []string{
			oidc.ScopeOpenID,
			oidc.ScopeProfile,
			oidc.ScopeEmail,
			"groups",
			oidc.ScopeOfflineAccess,
		},
		Provider: testProvider,
	}

	cache := NewK8sConfigCache(
		loginCfg,
		"cluster1-access",
		"cluster2-access",
	)

	test := func(configPath string, expectedErr string, expectedRefreshToken string) {
		t.Logf("Testing %s", configPath)

		cache.kubeConfigPath = configPath
		token, err := cache.Token()
		if expectedErr != "" {
			require.Error(t, err)
			assert.Equal(t, expectedErr, err.Error())
		} else {
			require.NoError(t, err)
			assert.Equal(t, expectedRefreshToken, token.RefreshToken)
		}
	}

	for _, c := range []struct {
		configPath           string
		expectedErrMsg       string
		expectedRefreshToken string
	}{
		{
			configPath:     "test-data/no_auth_config.yaml",
			expectedErrMsg: "No OIDC auth provider section for user cluster2-access",
		},
		{
			configPath:     "test-data/wrong_clientid_config.yaml",
			expectedErrMsg: "Wrong ClientID for user cluster2-access",
		},
		{
			configPath:     "test-data/wrong_clientsecret_config.yaml",
			expectedErrMsg: "Wrong ClientSecret for user cluster2-access",
		},
		{
			configPath:     "test-data/wrong_scopes_config.yaml",
			expectedErrMsg: "Extra scopes does not match for user cluster2-access",
		},
		{
			configPath:     "test-data/wrong_idp_config.yaml",
			expectedErrMsg: "Wrong Issuer Identity Provider for user cluster2-access",
		},
		{
			configPath:     "test-data/diff_refreshtoken_config.yaml",
			expectedErrMsg: "Different RefreshTokens among users, found on user cluster2-access",
		},
		{
			configPath:     "test-data/not_all_users_config.yaml",
			expectedErrMsg: "Failed to find all of the users. Found 1, need 2",
		},
		{
			configPath:           "test-data/ok_config.yaml",
			expectedRefreshToken: "refresh_token1",
		},
	} {
		test(c.configPath, c.expectedErrMsg, c.expectedRefreshToken)
	}
}

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

func TestK8sCache_SetToken(t *testing.T) {
	loginCfg := Config{
		ClientID:     "ID1",
		ClientSecret: "secret1",
		NonceCheck:   true,
		Scopes: []string{
			oidc.ScopeOpenID,
			oidc.ScopeProfile,
			oidc.ScopeEmail,
			"groups",
			oidc.ScopeOfflineAccess,
		},
		Provider: testProvider,
	}

	cache := NewK8sConfigCache(
		loginCfg,
		"cluster1-access",
		"cluster2-access",
	)

	test := func(inputCfgPath string) {
		t.Logf("Testing %s", inputCfgPath)
		cache.kubeConfigPath = "test-data/tmp-" + rand128Bits()

		err := copyFileContents(inputCfgPath, cache.kubeConfigPath)
		require.NoError(t, err)

		defer os.Remove(cache.kubeConfigPath)
		token := &oidc.Token{
			RefreshToken: "new-refresh-token",
			IDToken:      "new-id-token",
		}

		err = cache.SetToken(token)
		require.NoError(t, err)

		file, err := ioutil.ReadFile(cache.kubeConfigPath)
		require.NoError(t, err)

		expected, err := ioutil.ReadFile("test-data/expected_config.yaml")
		require.NoError(t, err)

		assert.Equal(t, string(expected), string(file))
	}

	for _, inputCfgPath := range []string{
		"test-data/no_auth_config.yaml",
		"test-data/wrong_clientid_config.yaml",
		"test-data/wrong_clientsecret_config.yaml",
		"test-data/wrong_scopes_config.yaml",
		"test-data/wrong_idp_config.yaml",
		"test-data/diff_refreshtoken_config.yaml",
		//"test-data/not_all_users_config.yaml", This needs to be excluded - we are not quarantining any order.
		"test-data/ok_config.yaml",
	} {
		test(inputCfgPath)
	}
}