-- Deposit money that actually arrived on a balance invoice, for a camper added
-- after the group's deposit checkout had already been paid. Settling the balance
-- moves deposit_owed_pence into this column rather than discarding it, so that
-- deleting the registration later can keep the deposit instead of handing it back
-- inside the balance refund.
--
-- Zero for everyone who paid their deposit at checkout: that money is the group's
-- total_amount_pence and is not repeated here. Also zero for deposits settled
-- before this column existed, which is not recoverable from our own data.
ALTER TABLE registrations
    ADD COLUMN deposit_paid_pence INT NOT NULL DEFAULT 0
    CHECK (deposit_paid_pence >= 0);
