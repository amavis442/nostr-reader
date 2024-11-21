--
-- TOC entry 223 (class 1259 OID 16431)
-- Name: profiles; Type: TABLE; Schema: public; Owner: nostr
--

CREATE TABLE IF NOT EXISTS public.profiles (
    id bigint NOT NULL,
    pubkey character varying(100) NOT NULL,
    name character varying(255),
    about text,
    picture character varying(255),
    website character varying(255),
    nip05 character varying(255),
    lud16 character varying(255),
    display_name character varying(255),
    raw jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone,
    followed boolean DEFAULT false NOT NULL,
    url character varying(255),
    blocked boolean DEFAULT false NOT NULL,
    uid character(36),
    urls text[],
    is_followed boolean
);

ALTER TABLE public.profiles OWNER TO nostr;

CREATE SEQUENCE public.profiles_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.profiles_id_seq OWNER TO nostr;
ALTER SEQUENCE public.profiles_id_seq OWNED BY public.profiles.id;