CREATE TABLE contacts (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    first_name TEXT        NOT NULL,
    last_name  TEXT        NOT NULL DEFAULT '',
    phone      TEXT        NOT NULL,
    email      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT contacts_phone_key UNIQUE (phone)
);

CREATE INDEX contacts_name_idx ON contacts (last_name, first_name);
