CREATE OR REPLACE VIEW notes_and_profiles
		AS
 		SELECT notes.id, notes.uid as note_uuid, notes.event_id, notes.pubkey, notes.kind, notes.event_created_at,
        notes.content, notes.tags_full::json, notes.sig, notes.etags, notes.ptags,
        profiles.uid as profile_uuid, profiles.name, profiles.about , profiles.picture,
        profiles.website, profiles.nip05, profiles.lud16, profiles.display_name,
        CASE WHEN length(follows.pubkey) > 0 THEN TRUE ELSE FALSE END followed,
        CASE WHEN length(bookmarks.event_id) > 0 THEN TRUE ELSE FALSE END bookmarked FROM "notes" 
		  LEFT JOIN profiles ON (profiles.pubkey = notes.pubkey) 
		  LEFT JOIN blocks  on (blocks.pubkey = notes.pubkey) 
		  LEFT JOIN bookmarks ON (bookmarks.note_id = notes.id) 
		  LEFT JOIN follows ON (follows.pubkey = notes.pubkey) 
		  WHERE notes.kind = 1 AND notes.garbage = false AND notes.root = true AND blocks.pubkey IS NULL ORDER BY notes.id asc;