/*
Copyright 2014 The Perkeep Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package serverconfig provides types related to the server configuration file.
package serverconfig // import "perkeep.org/pkg/types/serverconfig"

import (
	"encoding/json"
)

// Config holds the values from the JSON (high-level) server config
// file that is exposed to users (and is by default at
// osutil.UserServerConfigPath). From this simpler configuration, a
// complete, low-level one, is generated by
// serverinit.genLowLevelConfig, and used to configure the various
// Camlistore components.
type Config struct {
	Auth    string `json:"auth"`              // auth scheme and values (ex: userpass:foo:bar).
	BaseURL string `json:"baseURL,omitempty"` // Base URL the server advertizes. For when behind a proxy.
	Listen  string `json:"listen"`            // address (of the form host|ip:port) on which the server will listen on.

	// CamliNetIP is the optional internet-facing IP address for this
	// Camlistore instance. If set, a name in the camlistore.net domain for
	// that IP address will be requested on startup. The obtained domain name
	// will then be used as the host name in the base URL.
	// For now, the protocol to get the name requires receiving a challenge
	// on port 443. Also, this option implies HTTPS, and that the HTTPS
	// certificate is obtained from Let's Encrypt. For these reasons, this
	// option is mutually exclusive with BaseURL, Listen, HTTPSCert, and
	// HTTPSKey.
	CamliNetIP         string `json:"camliNetIP"`
	Identity           string `json:"identity"`           // GPG identity.
	IdentitySecretRing string `json:"identitySecretRing"` // path to the secret ring file.

	// alternative source tree, to override the embedded ui and/or closure resources.
	// If non empty, the ui files will be expected at
	// sourceRoot + "/server/camlistored/ui" and the closure library at
	// sourceRoot + "/vendor/embed/closure/lib"
	// Also used by the publish handler.
	SourceRoot string `json:"sourceRoot,omitempty"`

	// OwnerName is the full name of this Perkeep instance. Currently unused.
	OwnerName string `json:"ownerName,omitempty"`

	// Blob storage.
	MemoryStorage      bool   `json:"memoryStorage,omitempty"`      // do not store anything (blobs or queues) on localdisk, use memory instead.
	BlobPath           string `json:"blobPath,omitempty"`           // path to the directory containing the blobs.
	PackBlobs          bool   `json:"packBlobs,omitempty"`          // use "diskpacked" instead of the default filestorage. (exclusive with PackRelated)
	PackRelated        bool   `json:"packRelated,omitempty"`        // use "blobpacked" instead of the default storage (exclusive with PackBlobs)
	S3                 string `json:"s3,omitempty"`                 // Amazon S3 credentials: access_key_id:secret_access_key:bucket[/optional/dir][:hostname].
	B2                 string `json:"b2,omitempty"`                 // Backblaze B2 credentials: account_id:application_key:bucket[/optional/dir].
	GoogleCloudStorage string `json:"googlecloudstorage,omitempty"` // Google Cloud credentials: clientId:clientSecret:refreshToken:bucket[/optional/dir] or ":bucket[/optional/dir/]" for auto on GCE
	GoogleDrive        string `json:"googledrive,omitempty"`        // Google Drive credentials: clientId:clientSecret:refreshToken:parentId.
	ShareHandler       bool   `json:"shareHandler,omitempty"`       // enable the share handler. If true, and shareHandlerPath is empty then shareHandlerPath will default to "/share/" when generating the low-level config.
	ShareHandlerPath   string `json:"shareHandlerPath,omitempty"`   // URL prefix for the share handler. If set, overrides shareHandler.

	// HTTPS.
	HTTPS     bool   `json:"https,omitempty"`     // enable HTTPS.
	HTTPSCert string `json:"httpsCert,omitempty"` // path to the HTTPS certificate file.
	HTTPSKey  string `json:"httpsKey,omitempty"`  // path to the HTTPS key file.

	// Index.
	RunIndex          invertedBool `json:"runIndex,omitempty"`          // if logically false: no search, no UI, etc.
	CopyIndexToMemory invertedBool `json:"copyIndexToMemory,omitempty"` // copy disk-based index to memory on start-up.
	MemoryIndex       bool         `json:"memoryIndex,omitempty"`       // use memory-only indexer.

	// DBName is the optional name of the index database for MySQL, PostgreSQL, MongoDB.
	// If empty, DBUnique is used as part of the database name.
	DBName string `json:"dbname,omitempty"`

	// DBUnique optionally provides a unique value to differentiate databases on a
	// DBMS shared by multiple Perkeep instances. It should not contain spaces or
	// punctuation. If empty, Identity is used instead. If the latter is absent, the
	// current username (provided by the operating system) is used instead. For the
	// index database, DBName takes priority.
	DBUnique   string `json:"dbUnique,omitempty"`
	LevelDB    string `json:"levelDB,omitempty"`     // path to the levelDB directory, for indexing with github.com/syndtr/goleveldb.
	KVFile     string `json:"kvIndexFile,omitempty"` // path to the kv file, for indexing with github.com/cznic/kv.
	MySQL      string `json:"mysql,omitempty"`       // MySQL credentials (username@host:password), for indexing with MySQL.
	Mongo      string `json:"mongo,omitempty"`       // MongoDB credentials ([username:password@]host), for indexing with MongoDB.
	PostgreSQL string `json:"postgres,omitempty"`    // PostgreSQL credentials (username@host:password), for indexing with PostgreSQL.
	SQLite     string `json:"sqlite,omitempty"`      // path to the SQLite file, for indexing with SQLite.

	ReplicateTo []interface{} `json:"replicateTo,omitempty"` // NOOP for now.
	// Publish maps a URL prefix path used as a root for published paths (a.k.a. a camliRoot path), to the configuration of the publish handler that serves all the published paths under this root.
	Publish map[string]*Publish `json:"publish,omitempty"`
	ScanCab *ScanCab            `json:"scancab,omitempty"` // Scanning cabinet app configuration.

	// TODO(mpl): map of importers instead?
	Flickr string `json:"flickr,omitempty"` // flicker importer.
	Picasa string `json:"picasa,omitempty"` // picasa importer.
}

// App holds the common configuration values for apps and the app handler.
// See https://camlistore.org/doc/app-environment
type App struct {
	// Listen is the address (of the form host|ip:port) on which the app
	// will listen. It defines CAMLI_APP_LISTEN.
	// If empty, the default is the concatenation of the Camlistore server's
	// Listen host part, and a random port.
	Listen string `json:"listen,omitempty"`

	// BackendURL is the URL of the application's process, always ending in a
	// trailing slash. It is the URL that the app handler will proxy to when
	// getting requests for the concerned app.
	// If empty, the default is the concatenation of the Camlistore server's BaseURL
	// scheme, the Camlistore server's BaseURL host part, and the port of Listen.
	BackendURL string `json:"backendURL,omitempty"`

	// APIHost is URL prefix of the Camlistore server which the app should
	// use to make API calls. It defines CAMLI_API_HOST.
	// If empty, the default is the Camlistore server's BaseURL, with a
	// trailing slash appended.
	APIHost string `json:"apiHost,omitempty"`

	// HTTPSCert is the path to the HTTPS certificate file. If not set, and
	// Camlistore is using HTTPS, the app should try to use Camlistore's Let's
	// Encrypt cache (assuming it runs on the same host).
	HTTPSCert string `json:"httpsCert,omitempty"`
	HTTPSKey  string `json:"httpsKey,omitempty"` // path to the HTTPS key file.
}

// Publish holds the server configuration values specific to a publisher, i.e. to a publish prefix.
type Publish struct {
	// Program is the server app program to run as the publisher.
	// Defaults to "publisher".
	Program string `json:"program"`

	*App // Common apps and app handler configuration.

	// CamliRoot value that defines our root permanode for this
	// publisher. The root permanode is used as the root for all the
	// paths served by this publisher.
	CamliRoot string `json:"camliRoot"`

	// GoTemplate is the name of the Go template file used by this
	// publisher to represent the data. This file should live in
	// app/publisher/.
	GoTemplate string `json:"goTemplate"`

	// CacheRoot is the path that will be used as the root for the
	// caching blobserver (for images). No caching if empty.
	// An example value is Config.BlobPath + "/cache".
	CacheRoot string `json:"cacheRoot,omitempty"`

	// SourceRoot optionally defines the directory where to look for some resources
	// such as HTML templates, as well as javascript, and CSS files. The
	// default is to use the resources embedded in the publisher binary, found
	// in the publisher app source directory.
	SourceRoot string `json:"sourceRoot,omitempty"`
}

// ScanCab holds the server configuration values specific to a scanning cabinet
// app. Please note that the scanning cabinet app is still experimental and is
// subject to change.
type ScanCab struct {
	// Program is the server app program to run as the scanning cabinet.
	// Defaults to "scanningcabinet".
	Program string `json:"program"`

	// Prefix is the URL path prefix where the scanning cabinet app handler is mounted
	// on Camlistore.
	// It always ends with a trailing slash. Examples: "/scancab/", "/scanning/".
	Prefix string `json:"prefix"`

	// TODO(mpl): maybe later move Auth to type App. For now just in ScanCab as
	// publisher does not support any auth. Should be trivial to add though.

	// Auth is the authentication scheme and values to access the app.
	// It defaults to the server config auth.
	// Common uses are HTTP basic auth: "userpass:foo:bar", or no authentication:
	// "none". See https://camlistore.org/pkg/auth for other schemes.
	Auth string `json:"auth"`

	// App is for the common apps and app handler configuration.
	*App
}

// invertedBool is a bool that marshals to and from JSON with the opposite of its in-memory value.
type invertedBool bool

func (ib invertedBool) MarshalJSON() ([]byte, error) {
	return json.Marshal(!bool(ib))
}

func (ib *invertedBool) UnmarshalJSON(b []byte) error {
	var bo bool
	if err := json.Unmarshal(b, &bo); err != nil {
		return err
	}
	*ib = invertedBool(!bo)
	return nil
}

// Get returns the logical value of ib.
func (ib invertedBool) Get() bool {
	return !bool(ib)
}
