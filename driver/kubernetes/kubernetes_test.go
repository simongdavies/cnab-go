package kubernetes

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/cnabio/cnab-go/bundle"
	"github.com/cnabio/cnab-go/driver"
)

func TestDriver_Run(t *testing.T) {
	// Simulate the shared volume
	sharedDir, err := ioutil.TempDir("", "cnab-go")
	require.NoError(t, err, "could not create test directory")
	defer os.RemoveAll(sharedDir)

	client := fake.NewSimpleClientset()
	namespace := "default"
	k := Driver{
		Namespace:          namespace,
		jobs:               client.BatchV1().Jobs(namespace),
		secrets:            client.CoreV1().Secrets(namespace),
		pods:               client.CoreV1().Pods(namespace),
		JobVolumePath:      sharedDir,
		JobVolumeName:      "cnab-driver-shared",
		SkipCleanup:        true,
		skipJobStatusCheck: true,
	}
	op := driver.Operation{
		Action: "install",
		Bundle: &bundle.Bundle{},
		Image:  bundle.InvocationImage{BaseImage: bundle.BaseImage{Image: "foo/bar"}},
		Out:    os.Stdout,
		Environment: map[string]string{
			"foo": "bar",
		},
	}

	_, err = k.Run(&op)
	assert.NoError(t, err)

	jobList, _ := k.jobs.List(metav1.ListOptions{})
	assert.Equal(t, len(jobList.Items), 1, "expected one job to be created")

	secretList, _ := k.secrets.List(metav1.ListOptions{})
	assert.Equal(t, len(secretList.Items), 1, "expected one secret to be created")
}

func TestDriver_RunWithSharedFiles(t *testing.T) {
	// Simulate the shared volume
	sharedDir, err := ioutil.TempDir("", "cnab-go")
	require.NoError(t, err, "could not create test directory")
	defer os.RemoveAll(sharedDir)

	// Simulate that the bundle generated output "foo"
	err = os.Mkdir(filepath.Join(sharedDir, "outputs"), 0755)
	require.NoError(t, err, "could not create outputs directory")
	err = ioutil.WriteFile(filepath.Join(sharedDir, "outputs/foo"), []byte("foobar"), 0644)
	require.NoError(t, err, "could not write output foo")

	client := fake.NewSimpleClientset()
	namespace := "default"
	k := Driver{
		Namespace:          namespace,
		jobs:               client.BatchV1().Jobs(namespace),
		secrets:            client.CoreV1().Secrets(namespace),
		pods:               client.CoreV1().Pods(namespace),
		JobVolumePath:      sharedDir,
		JobVolumeName:      "cnab-driver-shared",
		SkipCleanup:        true,
		skipJobStatusCheck: true,
	}
	op := driver.Operation{
		Action: "install",
		Image:  bundle.InvocationImage{BaseImage: bundle.BaseImage{Image: "foo/bar"}},
		Bundle: &bundle.Bundle{
			Outputs: map[string]bundle.Output{
				"foo": {
					Definition: "foo",
					Path:       "/cnab/app/outputs/foo",
				},
			},
		},
		Out: os.Stdout,
		Outputs: map[string]string{
			"/cnab/app/outputs/foo": "foo",
		},
		Environment: map[string]string{
			"foo": "bar",
		},
		Files: map[string]string{
			"/cnab/app/someinput": "input value",
		},
	}

	opResult, err := k.Run(&op)
	require.NoError(t, err)

	jobList, _ := k.jobs.List(metav1.ListOptions{})
	assert.Equal(t, len(jobList.Items), 1, "expected one job to be created")

	secretList, _ := k.secrets.List(metav1.ListOptions{})
	assert.Equal(t, len(secretList.Items), 1, "expected one secret to be created")

	require.Contains(t, opResult.Outputs, "foo", "expected the foo output to be collected")
	assert.Equal(t, "foobar", opResult.Outputs["foo"], "invalid output value for foo ")

	wantInputFile := filepath.Join(sharedDir, "inputs/cnab/app/someinput")
	inputContents, err := ioutil.ReadFile(wantInputFile)
	require.NoErrorf(t, err, "could not read generated input file %s on shared volume", wantInputFile)
	assert.Equal(t, "input value", string(inputContents), "invalid input file contents")
}

func TestImageWithDigest(t *testing.T) {
	testCases := map[string]bundle.InvocationImage{
		"foo": {
			BaseImage: bundle.BaseImage{
				Image: "foo",
			},
		},
		"foo/bar": {
			BaseImage: bundle.BaseImage{
				Image: "foo/bar",
			},
		},
		"foo/bar:baz": {
			BaseImage: bundle.BaseImage{
				Image: "foo/bar:baz",
			},
		},
		"foo/bar:baz@sha256:9cfb3575ae5ff2b23ffa3c8e9514d818a9028a71b1d1e3b56b31937188a70b21": {
			BaseImage: bundle.BaseImage{
				Image:  "foo/bar:baz",
				Digest: "sha256:9cfb3575ae5ff2b23ffa3c8e9514d818a9028a71b1d1e3b56b31937188a70b21",
			},
		},
		"foo/fun@sha256:9cfb3575ae5ff2b23ffa3c8e9514d818a9028a71b1d1e3b56b31937188a70b21": {
			BaseImage: bundle.BaseImage{
				Image:  "foo/fun@sha256:9cfb3575ae5ff2b23ffa3c8e9514d818a9028a71b1d1e3b56b31937188a70b21",
				Digest: "",
			},
		},
		"taco/truck@sha256:9cfb3575ae5ff2b23ffa3c8e9514d818a9028a71b1d1e3b56b31937188a70b21": {
			BaseImage: bundle.BaseImage{
				Image:  "taco/truck",
				Digest: "sha256:9cfb3575ae5ff2b23ffa3c8e9514d818a9028a71b1d1e3b56b31937188a70b21",
			},
		},
		"foo/baz@sha256:9cfb3575ae5ff2b23ffa3c8e9514d818a9028a71b1d1e3b56b31937188a70b21": {
			BaseImage: bundle.BaseImage{
				Image:  "foo/baz@sha256:9cfb3575ae5ff2b23ffa3c8e9514d818a9028a71b1d1e3b56b31937188a70b21",
				Digest: "sha256:9cfb3575ae5ff2b23ffa3c8e9514d818a9028a71b1d1e3b56b31937188a70b21",
			},
		},
	}

	for expectedImageRef, img := range testCases {
		t.Run(expectedImageRef, func(t *testing.T) {
			img, err := imageWithDigest(img)
			require.NoError(t, err)
			assert.Equal(t, expectedImageRef, img)
		})
	}
}

func TestImageWithDigest_Failures(t *testing.T) {
	testcases := []struct {
		image     string
		digest    string
		wantError string
	}{
		{"foo/bar@sha:invalid", "",
			"could not parse foo/bar@sha:invalid as an OCI reference"},
		{"foo/bar:baz", "sha:invalid",
			"invalid digest sha:invalid specified for invocation image foo/bar:baz"},
		{"foo/bar@sha256:276f1974b4749003bc6c934593983314227cc9a1e6b922396fff59647b82dc4e", "sha256:176f1974b4749003bc6c934593983314227cc9a1e6b922396fff59647b82dc4e",
			"The digest sha256:176f1974b4749003bc6c934593983314227cc9a1e6b922396fff59647b82dc4e for the image foo/bar@sha256:276f1974b4749003bc6c934593983314227cc9a1e6b922396fff59647b82dc4e doesn't match the one specified in the image"},
	}

	for _, tc := range testcases {
		input := bundle.InvocationImage{}
		input.Image = tc.image
		input.Digest = tc.digest
		_, err := imageWithDigest(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), tc.wantError)
	}
}

func TestGenerateNameTemplate(t *testing.T) {
	testCases := map[string]struct {
		op       *driver.Operation
		expected string
	}{
		"short name": {
			op: &driver.Operation{
				Action:       "install",
				Installation: "foo",
			},
			expected: "install-foo-",
		},
		"special chars": {
			op: &driver.Operation{
				Action:       "example.com/liftoff",
				Installation: "🚀 me to the 🌙",
			},
			expected: "example.com-liftoff-me-to-the-",
		},
		"long installation name": {
			op: &driver.Operation{
				Action:       "install",
				Installation: "this-should-be-truncated-qcUYSfR9MS3BqR0kRDHe2K5EHJa8BJGrcoiDVvsDpATjIkr",
			},
			expected: "install-this-should-be-truncated-qcuysfr9ms3bqr0k-",
		},
		"maximum matching segments": {
			op: &driver.Operation{
				Action:       "a",
				Installation: "b c d e f g h i j k l m n o p q r s t u v w x y z",
			},
			expected: "a-b-c-d-e-f-g-h-i-j-k-l-m-n-o-p-q-r-s-t-u-v-w-x-y-",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := generateNameTemplate(tc.op)
			assert.Equal(t, tc.expected, actual)
			assert.True(t, len(actual) <= maxNameTemplateLength)
		})
	}
}

func TestDriver_SetConfig_Fails(t *testing.T) {
	t.Run("job volume name missing", func(t *testing.T) {

		d := Driver{}
		err := d.SetConfig(map[string]string{
			"JOB_VOLUME_PATH": "/tmp",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "setting JOB_VOLUME_NAME is required")
	})

	t.Run("job volume path missing", func(t *testing.T) {

		d := Driver{}
		err := d.SetConfig(map[string]string{
			"JOB_VOLUME_Name": "cnab-driver-shared",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "setting JOB_VOLUME_PATH is required")
	})

	t.Run("kubeconfig invalid", func(t *testing.T) {

		d := Driver{}
		err := d.SetConfig(map[string]string{
			"KUBECONFIG":      "invalid",
			"JOB_VOLUME_NAME": "cnab-driver-shared",
			"JOB_VOLUME_PATH": "/tmp",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error retrieving external kubernetes configuration using configuration")
	})

	t.Run("use in-cluster outside cluster", func(t *testing.T) {
		// Force this to fail even when the tests are run inside brigade
		orig := os.Getenv("KUBERNETES_SERVICE_HOST")
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		defer os.Setenv("KUBERNETES_SERVICE_HOST", orig)

		d := Driver{}
		err := d.SetConfig(map[string]string{
			"IN_CLUSTER":      "true",
			"JOB_VOLUME_NAME": "cnab-driver-shared",
			"JOB_VOLUME_PATH": "/tmp",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error retrieving in-cluster kubernetes configuration")
	})
}
