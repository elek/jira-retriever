
# jira-retriever

This simple script is do incremental import/notification from jira to db/slack.


## Description

Originally it used to mirror jira database to a sql database. Recently it's modified to save the incrementale change to different location including messaging apps.

It calls the jira API with a predefined _query_ since a _specific time_, save every issues/comments/changes from the results to the _destination_.

 * _query_: jira jsql could be defined by the `query` parameter

 * _specific time_: Could be nailed down with `since` parameter. By default the application saves the last modification time from the prevous run and it will be used by the next run.

   * For the SQL adapter it could be retrieved from the existing records. Next time the jsql should be limited to the records which are newer than the newest record in the db

	* For slack/console adapter: it saves the timestampe of the most recent records to the home directory and will use next time

## Available adapters

Current adapters:

```
Available Commands:
  console     Print out the latest changes to the console.
  slack       Send the latest changes to slack
  todb        Save latest changes to postgresql db.
```

Status: 

 * I use the previous version of the todb adapter in production. Latest version is not tested very well.
 * slack/console adapter is used in production and tested with multiple projects.

## Schema required by the todb adapter

```
CREATE TABLE issue
(
    key character varying COLLATE pg_catalog."default" NOT NULL,
    updated timestamp with time zone NOT NULL,
    selector character varying,
    value jsonb NOT NULL,
    CONSTRAINT jira_pkey PRIMARY KEY (key)
);

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
);

```
