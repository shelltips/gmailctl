package config

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/google/go-jsonnet"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	cfgv1 "github.com/mbrt/gmailctl/pkg/config/v1alpha1"
	cfgv2 "github.com/mbrt/gmailctl/pkg/config/v1alpha2"
	cfgv3 "github.com/mbrt/gmailctl/pkg/config/v1alpha3"
)

// LatestVersion points to the latest version of the config format.
const LatestVersion = cfgv3.Version

// ReadFile takes a path and returns the parsed config file.
//
// If the config file needs to have access to additional libraries,
// their location can be specified with cfgDirs.
func ReadFile(path, libPath string) (cfgv3.Config, error) {
	/* #nosec */
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return cfgv3.Config{}, NotFoundError(err)
	}
	if filepath.Ext(path) == ".jsonnet" {
		// We pass the libPath to jsonnet, because that is the hint
		// to the libraries location. If no library is specified,
		// we use the original file location.
		if libPath != "" {
			return readJsonnet(libPath, b)
		}
		return readJsonnet(path, b)
	}
	return readYaml(b)
}

func readJsonnet(path string, buf []byte) (cfgv3.Config, error) {
	var res cfgv3.Config
	vm := jsonnet.MakeVM()
	jstr, err := vm.EvaluateSnippet(path, string(buf))
	if err != nil {
		return res, errors.Wrap(err, "invalid jsonnet")
	}
	version, err := readJSONVersion(jstr)
	if err != nil {
		return res, errors.Wrap(err, "error parsing the config version")
	}

	switch version {
	case cfgv3.Version:
		err = jsonUnmarshalStrict([]byte(jstr), &res)
		return res, err

	case cfgv2.Version:
		var v2 cfgv2.Config
		err = jsonUnmarshalStrict([]byte(jstr), &v2)
		if err != nil {
			return res, errors.Wrap(err, "error parsing v1alpha2 config")
		}
		return importFromV2(v2)

	case cfgv1.Version:
		var v1 cfgv1.Config
		err = jsonUnmarshalStrict([]byte(jstr), &v1)
		if err != nil {
			return res, errors.Wrap(err, "error parsing v1alpha1 config")
		}
		return importFromV1(v1)

	default:
		return res, errors.Errorf("unknown config version: %s", version)
	}
}

func readYaml(buf []byte) (cfgv3.Config, error) {
	// TODO: Get rid of support for YAML config v3
	var res cfgv3.Config
	version, err := readYamlVersion(buf)
	if err != nil {
		return res, errors.Wrap(err, "error parsing the config version")
	}

	switch version {
	case cfgv3.Version:
		err = yaml.UnmarshalStrict(buf, &res)
		return res, err

	case cfgv2.Version:
		var v2 cfgv2.Config
		err = yaml.UnmarshalStrict(buf, &v2)
		if err != nil {
			return res, errors.Wrap(err, "error parsing v1alpha2 config")
		}
		return importFromV2(v2)

	case cfgv1.Version:
		var v1 cfgv1.Config
		err = yaml.UnmarshalStrict(buf, &v1)
		if err != nil {
			return res, errors.Wrap(err, "error parsing v1alpha1 config")
		}
		return importFromV1(v1)

	default:
		return res, errors.Errorf("unknown config version: %s", version)
	}
}

func importFromV1(v1 cfgv1.Config) (cfgv3.Config, error) {
	v2, err := cfgv2.Import(v1)
	if err != nil {
		return cfgv3.Config{}, err
	}
	return importFromV2(v2)
}

func importFromV2(v2 cfgv2.Config) (cfgv3.Config, error) {
	return cfgv3.Import(v2)
}

func readYamlVersion(buf []byte) (string, error) {
	// Try to unmarshal only the version
	v := struct {
		Version string `yaml:"version"`
	}{}
	err := yaml.Unmarshal(buf, &v)
	return v.Version, err
}

func readJSONVersion(js string) (string, error) {
	// Try to unmarshal only the version
	v := struct {
		Version string `json:"version"`
	}{}
	err := json.Unmarshal([]byte(js), &v)
	return v.Version, err
}

func jsonUnmarshalStrict(b []byte, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// IsNotFound returns true if an error is related to a file not found
func IsNotFound(err error) bool {
	nfErr, ok := errors.Cause(err).(notFound)
	return ok && nfErr.NotFound()
}

// NotFoundError wraps the given error and makes it into a not found one
func NotFoundError(err error) error {
	if err == nil {
		return nil
	}
	return notFoundError{err}
}

type notFound interface {
	NotFound() bool
}

type notFoundError struct {
	error
}

func (e notFoundError) Error() string  { return e.error.Error() }
func (e notFoundError) NotFound() bool { return true }
