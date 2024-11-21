package tag

import (
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestProcessTagsPubkeyToLong(t *testing.T) {
	ev := &nostr.Event{}

	_, _, _, _, _, err := ProcessTags(ev, "118cd39da270a800372ab7276a46b488cca3c40dd2b34f73b857fc8f72fae0f81234121")

	if err == nil || err.Error() != "incorrect pubkey to long. max 64" {
		t.Log("Pubkey is empty or is too long and should give an error")
		t.Fail()
	}
}

func TestProcessTags(t *testing.T) {
	tags := nostr.Tags{}
	tags1 := nostr.Tag{"e", "0000640f9cce22fb3dfb13204e0eca583f8419e162093efc9e0d734c91e58bcc", "", "root"}
	tags2 := nostr.Tag{"p", "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"}
	tags = append(tags, tags1)
	tags = append(tags, tags2)

	ev := &nostr.Event{
		ID:        "f0aa8df8e90cdb48cdb87b0cf1ba44b9f76f8f1cddd745aa8ef3ec19bc0c2647",
		PubKey:    "a1863ef588572c83daeb8946c47ed6a715ce0cdd79248fa3cd3f4183907d85f0",
		Kind:      1,
		CreatedAt: 1731276373,
		Sig:       "fa835e2eeabee7855b9b7ab791526510136af4bc22f561951a4f75f661499e07e1f443e1c40607a399131d42f3c63284300714526e60fa8c0c4de19e713de65f",
		Tags:      tags,
	}

	etags, ptags, hasNotification, isRoot, tree, _ := ProcessTags(ev, "118cd39da270a800372ab7276a46b488cca3c40dd2b34f73b857fc8f72fae0f8")

	if len(etags) != 1 {
		t.Log("number of etags should be 1")
		t.Fail()
	}
	if len(ptags) != 1 {
		t.Log("number of ptags should be 1")
		t.Fail()
	}
	if hasNotification != false {
		t.Log("Hasnotifications should be false")
		t.Fail()
	}
	if isRoot != false {
		t.Log("isRoot should be false")
		t.Fail()
	}
	if tree.RootTag != "0000640f9cce22fb3dfb13204e0eca583f8419e162093efc9e0d734c91e58bcc" {
		t.Log("tree.RootTag should be 0000640f9cce22fb3dfb13204e0eca583f8419e162093efc9e0d734c91e58bcc")
		t.Fail()
	}
}

func TestProcessTagsShouldHaveNotification(t *testing.T) {
	tags := nostr.Tags{}
	tags1 := nostr.Tag{"e", "0000640f9cce22fb3dfb13204e0eca583f8419e162093efc9e0d734c91e58bcc", "", "root"}
	tags2 := nostr.Tag{"p", "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"}
	tags = append(tags, tags1)
	tags = append(tags, tags2)

	ev := &nostr.Event{
		ID:        "f0aa8df8e90cdb48cdb87b0cf1ba44b9f76f8f1cddd745aa8ef3ec19bc0c2647",
		PubKey:    "a1863ef588572c83daeb8946c47ed6a715ce0cdd79248fa3cd3f4183907d85f0",
		Kind:      1,
		CreatedAt: 1731276373,
		Sig:       "fa835e2eeabee7855b9b7ab791526510136af4bc22f561951a4f75f661499e07e1f443e1c40607a399131d42f3c63284300714526e60fa8c0c4de19e713de65f",
		Tags:      tags,
	}

	etags, ptags, hasNotification, isRoot, tree, _ := ProcessTags(ev, "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d")

	if len(etags) != 1 {
		t.Log("number of etags should be 1")
		t.Fail()
	}
	if len(ptags) != 1 {
		t.Log("number of ptags should be 1")
		t.Fail()
	}
	if hasNotification != true {
		t.Log("Hasnotifications should be true")
		t.Fail()
	}
	if isRoot != false {
		t.Log("isRoot should be false")
		t.Fail()
	}
	if tree.RootTag != "0000640f9cce22fb3dfb13204e0eca583f8419e162093efc9e0d734c91e58bcc" {
		t.Log("tree.RootTag should be 0000640f9cce22fb3dfb13204e0eca583f8419e162093efc9e0d734c91e58bcc")
		t.Fail()
	}
}
