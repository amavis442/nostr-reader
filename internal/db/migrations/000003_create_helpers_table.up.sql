CREATE TYPE public.vote AS ENUM (
    'like',
    'dislike'
);

ALTER TYPE public.vote OWNER TO nostr;

CREATE FUNCTION public.delete_submission() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
	BEGIN  
  		IF NEW.kind=5 THEN
       		DELETE FROM notes WHERE ARRAY[event_id] && NEW.etags AND NEW.pubkey=pubkey;
    		RETURN NULL;
  		END IF;
  		RETURN NEW;
	END;
	$$;


ALTER FUNCTION public.delete_submission() OWNER TO nostr;

-- Block anoying user
CREATE TABLE public.blocks (
    id bigint NOT NULL,
    pubkey character varying(100) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone
);

ALTER TABLE public.blocks OWNER TO nostr;

CREATE SEQUENCE public.blocks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.blocks_id_seq OWNER TO nostr;
ALTER SEQUENCE public.blocks_id_seq OWNED BY public.blocks.id;

-- Bookmark interesting notes
CREATE TABLE public.bookmarks (
    id bigint NOT NULL,
    event_id character varying(100) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone,
    note_id bigint
);

ALTER TABLE public.bookmarks OWNER TO nostr;
CREATE SEQUENCE public.bookmarks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.bookmarks_id_seq OWNER TO nostr;
ALTER SEQUENCE public.bookmarks_id_seq OWNED BY public.bookmarks.id;

-- Follow interesting users
CREATE TABLE public.follows (
    id bigint NOT NULL,
    pubkey character varying(100),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone
);
ALTER TABLE public.follows OWNER TO nostr;
CREATE SEQUENCE public.follows_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.follows_id_seq OWNER TO nostr;
ALTER SEQUENCE public.follows_id_seq OWNED BY public.follows.id;

-- Notications for you (someone responded on your post)
CREATE TABLE public.notifications (
    id bigint NOT NULL,
    note_id bigint NOT NULL,
    seen boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone
);
ALTER TABLE public.notifications OWNER TO nostr;
CREATE SEQUENCE public.notifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.notifications_id_seq OWNER TO nostr;
ALTER SEQUENCE public.notifications_id_seq OWNED BY public.notifications.id;

-- The likes and dislikes
CREATE TABLE public.reactions (
    id bigint NOT NULL,
    pubkey text NOT NULL,
    content text NOT NULL,
    current_vote public.vote NOT NULL,
    target_event_id text NOT NULL,
    from_event_id text NOT NULL,
    note_id bigint,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone
);
ALTER TABLE public.reactions OWNER TO nostr;
CREATE SEQUENCE public.reactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.reactions_id_seq OWNER TO nostr;
ALTER SEQUENCE public.reactions_id_seq OWNED BY public.reactions.id;

-- Which realays should be used for getting the notes and profiles
CREATE TABLE public.relays (
    id bigint NOT NULL,
    url character varying(255) NOT NULL,
    read boolean DEFAULT false,
    write boolean DEFAULT false,
    search boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone
);
ALTER TABLE public.relays OWNER TO nostr;
CREATE SEQUENCE public.relays_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.relays_id_seq OWNER TO nostr;
ALTER SEQUENCE public.relays_id_seq OWNED BY public.relays.id;

-- Which notes was last seen
CREATE TABLE public.seens (
    id bigint NOT NULL,
    event_id character varying(100) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone,
    note_id bigint
);
ALTER TABLE public.seens OWNER TO nostr;
CREATE SEQUENCE public.seens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.seens_id_seq OWNER TO nostr;
ALTER SEQUENCE public.seens_id_seq OWNED BY public.seens.id;


-- Keep track of the replies on notes
CREATE TABLE public.trees (
    id bigint NOT NULL,
    event_id character varying(100) NOT NULL,
    root_event_id character varying(100) NOT NULL,
    reply_event_id character varying(100) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone
);
ALTER TABLE public.trees OWNER TO nostr;
CREATE SEQUENCE public.trees_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.trees_id_seq OWNER TO nostr;
ALTER SEQUENCE public.trees_id_seq OWNED BY public.trees.id;


--
-- TOC entry 4737 (class 2604 OID 16463)
-- Name: blocks id; Type: DEFAULT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.blocks ALTER COLUMN id SET DEFAULT nextval('public.blocks_id_seq'::regclass);


--
-- TOC entry 4739 (class 2604 OID 16464)
-- Name: bookmarks id; Type: DEFAULT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.bookmarks ALTER COLUMN id SET DEFAULT nextval('public.bookmarks_id_seq'::regclass);


--
-- TOC entry 4741 (class 2604 OID 16465)
-- Name: follows id; Type: DEFAULT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.follows ALTER COLUMN id SET DEFAULT nextval('public.follows_id_seq'::regclass);


--
-- TOC entry 4743 (class 2604 OID 16466)
-- Name: notes id; Type: DEFAULT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.notes ALTER COLUMN id SET DEFAULT nextval('public.notes_id_seq'::regclass);


--
-- TOC entry 4762 (class 2604 OID 209451)
-- Name: notifications id; Type: DEFAULT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.notifications ALTER COLUMN id SET DEFAULT nextval('public.notifications_id_seq'::regclass);


--
-- TOC entry 4747 (class 2604 OID 639211)
-- Name: profiles id; Type: DEFAULT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.profiles ALTER COLUMN id SET DEFAULT nextval('public.profiles_id_seq1'::regclass);


--
-- TOC entry 4751 (class 2604 OID 16468)
-- Name: reactions id; Type: DEFAULT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.reactions ALTER COLUMN id SET DEFAULT nextval('public.reactions_id_seq'::regclass);


--
-- TOC entry 4753 (class 2604 OID 16469)
-- Name: relays id; Type: DEFAULT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.relays ALTER COLUMN id SET DEFAULT nextval('public.relays_id_seq'::regclass);


--
-- TOC entry 4758 (class 2604 OID 16470)
-- Name: seens id; Type: DEFAULT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.seens ALTER COLUMN id SET DEFAULT nextval('public.seens_id_seq'::regclass);


--
-- TOC entry 4760 (class 2604 OID 16471)
-- Name: trees id; Type: DEFAULT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.trees ALTER COLUMN id SET DEFAULT nextval('public.trees_id_seq'::regclass);


--
-- TOC entry 4766 (class 2606 OID 112434)
-- Name: blocks blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.blocks
    ADD CONSTRAINT blocks_pkey PRIMARY KEY (id);


--
-- TOC entry 4768 (class 2606 OID 112436)
-- Name: blocks blocks_pubkey_key; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.blocks
    ADD CONSTRAINT blocks_pubkey_key UNIQUE (pubkey);


--
-- TOC entry 4771 (class 2606 OID 112438)
-- Name: bookmarks bookmarks_event_id_key; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.bookmarks
    ADD CONSTRAINT bookmarks_event_id_key UNIQUE (event_id);


--
-- TOC entry 4773 (class 2606 OID 112440)
-- Name: bookmarks bookmarks_pkey; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.bookmarks
    ADD CONSTRAINT bookmarks_pkey PRIMARY KEY (id);


--
-- TOC entry 4776 (class 2606 OID 112442)
-- Name: follows follows_pkey; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_pkey PRIMARY KEY (id);


--
-- TOC entry 4778 (class 2606 OID 112444)
-- Name: follows follows_pubkey_key; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_pubkey_key UNIQUE (pubkey);


--
-- TOC entry 4788 (class 2606 OID 112446)
-- Name: notes notes_event_id_key; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.notes
    ADD CONSTRAINT notes_event_id_key UNIQUE (event_id);


--
-- TOC entry 4790 (class 2606 OID 112449)
-- Name: notes notes_pkey; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.notes
    ADD CONSTRAINT notes_pkey PRIMARY KEY (id);


--
-- TOC entry 4823 (class 2606 OID 209455)
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- TOC entry 4795 (class 2606 OID 639213)
-- Name: profiles profiles_pkey1; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.profiles
    ADD CONSTRAINT profiles_pkey1 PRIMARY KEY (id);


--
-- TOC entry 4797 (class 2606 OID 112456)
-- Name: profiles profiles_pubkey_key; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.profiles
    ADD CONSTRAINT profiles_pubkey_key UNIQUE (pubkey);


--
-- TOC entry 4801 (class 2606 OID 112458)
-- Name: reactions reactions_pkey; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.reactions
    ADD CONSTRAINT reactions_pkey PRIMARY KEY (id);


--
-- TOC entry 4803 (class 2606 OID 112460)
-- Name: reactions reactions_pubkey_key; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.reactions
    ADD CONSTRAINT reactions_pubkey_key UNIQUE (pubkey);


--
-- TOC entry 4805 (class 2606 OID 112463)
-- Name: reactions reactions_target_event_id_key; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.reactions
    ADD CONSTRAINT reactions_target_event_id_key UNIQUE (target_event_id);


--
-- TOC entry 4807 (class 2606 OID 112465)
-- Name: relays relays_pkey; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.relays
    ADD CONSTRAINT relays_pkey PRIMARY KEY (id);


--
-- TOC entry 4809 (class 2606 OID 112467)
-- Name: relays relays_url_key; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.relays
    ADD CONSTRAINT relays_url_key UNIQUE (url);


--
-- TOC entry 4812 (class 2606 OID 112469)
-- Name: seens seens_event_id_key; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.seens
    ADD CONSTRAINT seens_event_id_key UNIQUE (event_id);


--
-- TOC entry 4814 (class 2606 OID 112471)
-- Name: seens seens_pkey; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.seens
    ADD CONSTRAINT seens_pkey PRIMARY KEY (id);


--
-- TOC entry 4819 (class 2606 OID 112473)
-- Name: trees trees_event_id_key; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.trees
    ADD CONSTRAINT trees_event_id_key UNIQUE (event_id);


--
-- TOC entry 4821 (class 2606 OID 112475)
-- Name: trees trees_pkey; Type: CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.trees
    ADD CONSTRAINT trees_pkey PRIMARY KEY (id);


--
-- TOC entry 4769 (class 1259 OID 112476)
-- Name: idx_blocks_pubkey; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_blocks_pubkey ON public.blocks USING btree (pubkey);


--
-- TOC entry 4774 (class 1259 OID 112477)
-- Name: idx_bookmarks_event_id; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_bookmarks_event_id ON public.bookmarks USING btree (event_id);


--
-- TOC entry 4779 (class 1259 OID 112478)
-- Name: idx_follows_pubkey; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_follows_pubkey ON public.follows USING btree (pubkey);


--
-- TOC entry 4780 (class 1259 OID 112479)
-- Name: idx_notes_etags; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_notes_etags ON public.notes USING gin (etags);


--
-- TOC entry 4781 (class 1259 OID 112480)
-- Name: idx_notes_event_id; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_notes_event_id ON public.notes USING btree (event_id);


--
-- TOC entry 4782 (class 1259 OID 197892)
-- Name: idx_notes_kind; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_notes_kind ON public.notes USING btree (kind);


--
-- TOC entry 4783 (class 1259 OID 112481)
-- Name: idx_notes_ptags; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_notes_ptags ON public.notes USING gin (ptags);


--
-- TOC entry 4784 (class 1259 OID 112482)
-- Name: idx_notes_pubkey; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_notes_pubkey ON public.notes USING btree (pubkey);


--
-- TOC entry 4785 (class 1259 OID 196845)
-- Name: idx_notes_root; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_notes_root ON public.notes USING btree (root);


--
-- TOC entry 4786 (class 1259 OID 648582)
-- Name: idx_notes_urls; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_notes_urls ON public.notes USING gin (urls);


--
-- TOC entry 4791 (class 1259 OID 112483)
-- Name: idx_profile_pubkey; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_profile_pubkey ON public.profiles USING btree (pubkey);


--
-- TOC entry 4792 (class 1259 OID 648811)
-- Name: idx_profile_urls; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_profile_urls ON public.profiles USING gin (urls);


--
-- TOC entry 4793 (class 1259 OID 112484)
-- Name: idx_profiles_pubkey; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_profiles_pubkey ON public.profiles USING btree (pubkey);


--
-- TOC entry 4798 (class 1259 OID 112485)
-- Name: idx_reactions_from_event_id; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_reactions_from_event_id ON public.reactions USING btree (from_event_id);


--
-- TOC entry 4810 (class 1259 OID 112486)
-- Name: idx_seens_event_id; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_seens_event_id ON public.seens USING btree (event_id);


--
-- TOC entry 4815 (class 1259 OID 112487)
-- Name: idx_trees_event_id; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_trees_event_id ON public.trees USING btree (event_id);


--
-- TOC entry 4816 (class 1259 OID 112488)
-- Name: idx_trees_reply_event_id; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_trees_reply_event_id ON public.trees USING btree (reply_event_id);


--
-- TOC entry 4817 (class 1259 OID 112489)
-- Name: idx_trees_root_event_id; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_trees_root_event_id ON public.trees USING btree (root_event_id);


--
-- TOC entry 4799 (class 1259 OID 112490)
-- Name: idx_vote_tables_pubkey_target; Type: INDEX; Schema: public; Owner: nostr
--

CREATE INDEX idx_vote_tables_pubkey_target ON public.reactions USING btree (pubkey, target_event_id);


--
-- TOC entry 4827 (class 2620 OID 659862)
-- Name: notes delete_trigger; Type: TRIGGER; Schema: public; Owner: nostr
--

CREATE TRIGGER delete_trigger BEFORE INSERT ON public.notes FOR EACH ROW EXECUTE FUNCTION public.delete_submission();


--
-- TOC entry 4825 (class 2606 OID 112492)
-- Name: reactions fk_notes_reaction; Type: FK CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.reactions
    ADD CONSTRAINT fk_notes_reaction FOREIGN KEY (note_id) REFERENCES public.notes(id);


--
-- TOC entry 4826 (class 2606 OID 209458)
-- Name: notifications fk_notifications_note; Type: FK CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT fk_notifications_note FOREIGN KEY (note_id) REFERENCES public.notes(id);


--
-- TOC entry 4824 (class 2606 OID 656235)
-- Name: notes fk_profiles_notes; Type: FK CONSTRAINT; Schema: public; Owner: nostr
--

ALTER TABLE ONLY public.notes
    ADD CONSTRAINT fk_profiles_notes FOREIGN KEY (profile_id) REFERENCES public.profiles(id);


-- Completed on 2024-10-21 21:34:41

--
-- PostgreSQL database dump complete
--
