/*
Copyright 2017 The Perkeep Authors.

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

// Package sharebutton provides a Button element that is used in the sidebar of
// the web UI, to share the selected items with a share claim. On success, the
// URL that can be used to share the items is displayed in a dialog. If the one
// item is a file, the URL can be used directly to fetch the file. If the one item
// is a directory, the URL should be used with pk-get -shared. If several (file or
// directory) items are selected, a new directory blob containing these items is
// created, and is the item getting shared instead.
package sharebutton

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gopherjs/gopherjs/js"

	"perkeep.org/pkg/auth"
	"perkeep.org/pkg/blob"
	"perkeep.org/pkg/client"
	"perkeep.org/pkg/schema"

	"honnef.co/go/js/dom"
	"myitcv.io/react"
)

//go:generate reactGen

// TODO(mpl): eventually, see what can be refactored with downloadbutton. But
// after I'm completely done with both of them (in other CLs).

// New returns the button element. It should be used as the entry point, to
// create the needed React element.
//
// key is the id for when the button is in a list, see
// https://facebook.github.io/react/docs/lists-and-keys.html
//
// config is the web UI config that was fetched from the server.
//
// cbs is a wrapper around the callback functions required by this component.
func New(key string, config map[string]string, cbs *Callbacks) react.Element {
	if config == nil {
		fmt.Println("Nil config for ShareItemsBtn")
		return nil
	}
	shareRoot, ok := config["shareRoot"]
	if !ok || shareRoot == "" {
		// Server has no share handler
		return nil
	}
	authToken, ok := config["authToken"]
	if !ok {
		fmt.Println("No authToken in config for ShareItemsBtn")
		return nil
	}
	uiRoot, ok := config["uiRoot"]
	if !ok {
		fmt.Println("No uiRoot in config for ShareItemsBtn")
		return nil
	}
	if key == "" {
		// A key is only needed in the context of a list, which is why
		// it is up to the caller to choose it. Just creating it here for
		// the sake of consistency.
		key = "shareItemsButton"
	}
	if cbs == nil {
		fmt.Println("Nil callbacks for ShareItemsBtn")
		return nil
	}
	if cbs.ShowSharedURL == nil {
		fmt.Println("Nil showSharedURL callback for ShareItemsBtn")
		return nil
	}
	if cbs.GetSelection == nil {
		fmt.Println("Nil getSelection callback for ShareItemsBtn")
		return nil
	}

	props := ShareItemsBtnProps{
		key:       key,
		callbacks: cbs,
		authToken: authToken,
		uiRoot:    uiRoot,
	}
	return ShareItemsBtn(props)
}

// Callbacks defines the callbacks that must be provided when creating a
// ShareItemsBtn instance.
type Callbacks struct {
	o *js.Object

	// GetSelection returns the list of items selected for sharing.
	GetSelection func() []SharedItem `js:"getSelection"`

	// ShowSharedURL displays in a dialog an anchor with anchorURL for its
	// href and anchorText for its text.
	ShowSharedURL func(anchorURL string, anchorText string) `js:"showSharedURL"`
}

// ShareItemsBtnDef is the definition for the button, that Renders as a React
// Button.
type ShareItemsBtnDef struct {
	react.ComponentDef
}

// SharedItem's only purpose is of documentation, since we can't enforce the
// type and the fields of what we get from javascript through GetSelection.
// A SharedItem's expected keys are:
//   "blobRef": "sha1-foo",
//   "isDir": "boolString",
// where "sha1-foo" is the ref of a file or a dir to share.
// and the value for "isDir", interpreted as a boolean with strconv, reports
// whether the ref is of a dir.
type SharedItem map[string]string

type ShareItemsBtnProps struct {
	// Key is the id for when the button is in a list, see
	// https://facebook.github.io/react/docs/lists-and-keys.html
	key string

	callbacks *Callbacks

	authToken string
	// uiRoot is used, with respect to the current window location, to figure
	// out the server's URL prefix.
	uiRoot string
}

func ShareItemsBtn(p ShareItemsBtnProps) *ShareItemsBtnElem {
	return buildShareItemsBtnElem(p)
}

func (d ShareItemsBtnDef) Render() react.Element {
	return react.Button(
		&react.ButtonProps{
			Key:     d.Props().key,
			OnClick: d,
		},
		react.S("Share"),
	)
}

// On success, handleShareSelection calls d.showSharedURL with the URL that can
// be used to share the item. If the item is a file, the URL can be used directly
// to fetch the file. If the item is a directory, the URL should be used with
// pk-get -shared.
func (d ShareItemsBtnDef) OnClick(e *react.SyntheticMouseEvent) {
	go func() {
		sharedURL, err := d.shareSelection()
		if err != nil {
			dom.GetWindow().Alert(fmt.Sprintf("%v", err))
			return
		}
		prefix, err := d.urlPrefix()
		if err != nil {
			dom.GetWindow().Alert(fmt.Sprintf("Cannot display full share URL: %v", err))
			return
		}
		sharedURL = prefix + sharedURL
		anchorText := sharedURL[:20] + "..." + sharedURL[len(sharedURL)-20:]
		// TODO(mpl): move some of the Dialog code to Go.
		d.Props().callbacks.ShowSharedURL(sharedURL, anchorText)
	}()
}

func (d ShareItemsBtnDef) shareSelection() (string, error) {
	selection := d.Props().callbacks.GetSelection()
	authToken := d.Props().authToken
	am, err := auth.NewTokenAuth(authToken)
	if err != nil {
		return "", fmt.Errorf("Error setting up auth for share request: %v", err)
	}
	var fileRef []blob.Ref
	isDir := false
	for _, item := range selection {
		br, ok := item["blobRef"]
		if !ok {
			return "", fmt.Errorf("cannot share item, it's missing a blobRef")
		}
		fbr, ok := blob.Parse(br)
		if !ok {
			return "", fmt.Errorf("cannot share %q, not a valid blobRef", br)
		}
		fileRef = append(fileRef, fbr)
		isDir, err = strconv.ParseBool(item["isDir"])
		if err != nil {
			return "", fmt.Errorf("invalid boolean value %q for isDir: %v", item["isDir"], err)
		}
	}
	if len(fileRef) == 1 {
		return shareFile(am, fileRef[0], isDir)
	}
	newDirbr, err := mkdir(am, fileRef)
	if err != nil {
		return "", fmt.Errorf("failed creating new directory for selected items: %v", err)
	}
	// TODO(mpl): should we bother deleting the dir and static set if
	// there's any failure from this point on? I think that as long as there's
	// no share claim referencing them, they're supposed to be GCed eventually,
	// aren't they? in which case, no need to worry about it.
	return shareFile(am, newDirbr, true)
}

// mkdir creates a new directory blob, with children composing its static-set,
// and uploads it. It returns the blobRef of the new directory.
func mkdir(am auth.AuthMode, children []blob.Ref) (blob.Ref, error) {
	cl, err := client.New(client.OptionAuthMode(am))
	if err != nil {
		return blob.Ref{}, err
	}
	var newdir blob.Ref
	ss := schema.NewStaticSet()
	subsets := ss.SetStaticSetMembers(children)
	for _, v := range subsets {
		// TODO(mpl): make them concurrent
		if _, err := cl.UploadBlob(context.TODO(), v); err != nil {
			return newdir, err
		}
	}
	ssb := ss.Blob()
	if _, err := cl.UploadBlob(context.TODO(), ssb); err != nil {
		return newdir, err
	}
	const fileNameLayout = "20060102150405"
	fileName := "shared-" + time.Now().Format(fileNameLayout)
	dir := schema.NewDirMap(fileName).PopulateDirectoryMap(ssb.BlobRef())
	dirBlob := dir.Blob()
	if _, err := cl.UploadBlob(context.TODO(), dirBlob); err != nil {
		return newdir, err
	}

	return dirBlob.BlobRef(), nil
}

// shareFile returns the URL that can be used to share the target item. If the
// item is a file, the URL can be used directly to fetch the file. If the item is a
// directory, the URL should be used with pk-get -shared.
func shareFile(am auth.AuthMode, target blob.Ref, isDir bool) (string, error) {
	cl, err := client.New(client.OptionAuthMode(am))
	if err != nil {
		return "", err
	}
	claim, err := newShareClaim(cl, target)
	if err != nil {
		return "", err
	}
	shareRoot, err := cl.ShareRoot()
	if err != nil {
		return "", err
	}
	if isDir {
		return fmt.Sprintf("%s%s", shareRoot, claim), nil
	}
	return fmt.Sprintf("%s%s?via=%s&assemble=1", shareRoot, target, claim), nil
}

// newShareClaim creates, signs, and uploads a transitive haveref share claim
// for sharing the target item. It returns the ref of the claim.
func newShareClaim(cl *client.Client, target blob.Ref) (blob.Ref, error) {
	var claim blob.Ref
	signer, err := cl.ServerPublicKeyBlobRef()
	if err != nil {
		return claim, fmt.Errorf("could not get signer: %v", err)
	}
	shareSchema := schema.NewShareRef(schema.ShareHaveRef, true)
	shareSchema.SetShareTarget(target)
	unsignedClaim, err := shareSchema.SetSigner(signer).JSON()
	if err != nil {
		return claim, fmt.Errorf("could not create unsigned share claim: %v", err)
	}
	signedClaim, err := cl.Sign(context.TODO(), "", strings.NewReader("json="+unsignedClaim))
	if err != nil {
		return claim, fmt.Errorf("could not get signed share claim: %v", err)
	}
	sbr, err := cl.Upload(context.TODO(), client.NewUploadHandleFromString(string(signedClaim)))
	if err != nil {
		return claim, fmt.Errorf("could not upload share claim: %v", err)
	}
	return sbr.BlobRef, nil
}

func (d ShareItemsBtnDef) urlPrefix() (string, error) {
	currentURL := dom.GetWindow().Location().Href
	uiRoot := d.Props().uiRoot
	if strings.HasSuffix(currentURL, uiRoot) {
		return strings.TrimSuffix(currentURL, uiRoot), nil
	}
	idx := strings.Index(currentURL, uiRoot)
	if idx == -1 {
		return "", fmt.Errorf("could not guess our URL prefix")
	}
	return currentURL[:idx], nil
}
