-- up
CREATE TABLE download_results(
        created_at TIMESTAMP NOT NULL DEFAULT now(),
	url        TEXT,
	success    boolean
);
