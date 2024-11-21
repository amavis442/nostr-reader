ALTER SCHEMA public OWNER TO nostr;

CREATE IF NOT EXISTS TABLE public.notes (
    id bigint NOT NULL,
    event_id text NOT NULL,
    pubkey character varying(100) NOT NULL,
    kind int NOT NULL,
    event_created_at bigint NOT NULL,
    content text,
    tags_full text,
    ptags text[],
    etags text[],
    sig character varying(200) NOT NULL,
    garbage boolean DEFAULT false,
    raw jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone,
    root boolean DEFAULT false,
    uid uuid,
    urls text[],
    profile_id bigint
);


ALTER TABLE public.notes OWNER TO nostr;
COMMENT ON COLUMN public.notes.root IS 'Is this the root note';

CREATE SEQUENCE public.notes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.notes_id_seq OWNER TO nostr;

ALTER SEQUENCE public.notes_id_seq OWNED BY public.notes.id;