package tag

import (
	"errors"
	"log"

	"github.com/nbd-wtf/go-nostr"
)

type EventTree struct {
	RootTag  string
	ReplyTag string
}

// Processes the #tags into etags, ptags and see which is the root and which is the reply
func ProcessTags(ev *nostr.Event, pubkey string) (etags []string, ptags []string, hasNotification bool, isRoot bool, tree EventTree, err error) {
	ptags, etags = make([]string, 0), make([]string, 0)
	isRoot = true

	if len(ev.PubKey) != 64 {
		err = errors.New("incorrect pubkey to long. max 64")
		//fmt.Println("Incorrect pubkey to long max 64: ", ev.PubKey, " Content:", ev.Content)
	}
	ptags = ptags[:0]
	etags = etags[:0]

	tree.RootTag = ""
	tree.ReplyTag = ""
	hasNotification = false
	hasRootOrReplyTag := false
	for _, tag := range ev.Tags {
		switch {
		case tag[0] == "e":
			if len(tag) < 1 || len(tag[1]) != 64 {
				continue
			} else {
				etags = append(etags, tag[1])
			}
			if len(tag) == 4 && tag[3] == "root" {
				tree.RootTag = tag[1]
				isRoot = false
				hasRootOrReplyTag = true
			}
			if len(tag) == 4 && tag[3] == "reply" {
				tree.ReplyTag = tag[1]
				isRoot = false
				hasRootOrReplyTag = true
			}
		case tag[0] == "p":
			if len(tag) < 1 || len(tag[1]) != 64 {
				log.Println("Tag:: P# tag not valid: ", tag)
				continue
			} else {
				ptags = append(ptags, tag[1])
				if tag[1] == pubkey && ev.PubKey != pubkey {
					hasNotification = true
				}
			}
		default:
			continue
		}
	}

	if len(etags) > 0 && !hasRootOrReplyTag {
		tree.RootTag = etags[0]
		if len(etags) > 1 {
			tree.ReplyTag = etags[len(etags)-1]
		}
		isRoot = false
	}

	return etags, ptags, hasNotification, isRoot, tree, err
}
