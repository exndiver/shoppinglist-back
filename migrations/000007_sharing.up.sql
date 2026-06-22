-- List sharing: membership + one-time invitations.
--
-- Model:
--   * A shopping_list is owned by shopping_lists.owner_id (the author).
--   * list_shares grants another owner (member_owner_id) access to that list,
--     either 'view' or 'edit'. The member may rename the list locally via
--     display_name without touching the canonical name.
--   * list_invitations are single-use, opaque, url-safe tokens. Accepting one
--     creates/refreshes a list_shares row and flips the token to 'accepted';
--     a second accept is rejected at the application layer.

CREATE TABLE IF NOT EXISTS list_shares (
    id              uuid PRIMARY KEY,
    list_id         uuid NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
    owner_id        uuid NOT NULL,                -- list author (sharer), denormalized for scoping
    member_owner_id uuid NOT NULL,                -- recipient's device/owner uuid
    access          text NOT NULL CHECK (access IN ('view', 'edit')),
    display_name    text,                         -- member's local rename; NULL = use list.name
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    revoked_at      timestamptz,
    UNIQUE (list_id, member_owner_id)
);

-- Lookups: lists a member can see, members of a list, and delta sync.
CREATE INDEX IF NOT EXISTS idx_list_shares_member_active
    ON list_shares (member_owner_id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_list_shares_list_active
    ON list_shares (list_id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_list_shares_member_updated
    ON list_shares (member_owner_id, updated_at);

CREATE TABLE IF NOT EXISTS list_invitations (
    token       text PRIMARY KEY,                 -- opaque url-safe token
    list_id     uuid NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
    owner_id    uuid NOT NULL,                    -- creator (list author)
    access      text NOT NULL CHECK (access IN ('view', 'edit')),
    status      text NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'accepted', 'revoked')),
    accepted_by uuid,                             -- recipient owner uuid, once accepted
    accepted_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz
);

CREATE INDEX IF NOT EXISTS idx_list_invitations_list
    ON list_invitations (list_id);
