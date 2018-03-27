Simple golang script to retrieve date from jira API incrementally.

It calls the jira api, save every issue from the results to postgresql based on the key as json type.

Next time it will query newest updated field and query only the updated issues.

A selector could be used to differentiate multiple query.


```

CREATE TABLE jiraissues
(
    key character varying COLLATE pg_catalog."default" NOT NULL,
    updated timestamp with time zone NOT NULL,
    selector character varying,
    value jsonb NOT NULL,
    CONSTRAINT jira_pkey PRIMARY KEY (key)
)

CREATE TABLE change
(
    id SERIAL,
    created timestamp with time zone NOT NULL,
    selector character varying,
    toString character varying,
    fromString character varying,
    field character varying,
    author_name character varying,
    author_key character varying,
    history_id int,
    item_index int,
    CONSTRAINT change_pid PRIMARY KEY (id)
)

```
