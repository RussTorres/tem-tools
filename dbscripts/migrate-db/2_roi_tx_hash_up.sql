ALTER TABLE roi_images ADD tx_hash VARCHAR(80);

CREATE INDEX roi_images_tx_hash_idx ON roi_images(tx_hash);
