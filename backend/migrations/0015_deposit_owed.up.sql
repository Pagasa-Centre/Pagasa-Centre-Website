-- Deposit a camper still owes because they were added after registration, when
-- the group's one-off deposit checkout had already been paid. Zero means nothing
-- outstanding, which is true for everyone who registered through the public form.
ALTER TABLE registrations
    ADD COLUMN deposit_owed_pence INT NOT NULL DEFAULT 0
    CHECK (deposit_owed_pence >= 0);
