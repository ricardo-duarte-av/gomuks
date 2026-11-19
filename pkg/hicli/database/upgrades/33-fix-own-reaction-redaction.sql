-- v33 (compatible with v10+): Fix own reaction removal storing arrays as strings
-- Newer SQLite versions drop the JSON subtype of scalar subquery results, so the
-- json_group_array output must be wrapped in json() to be inserted as an array.
DROP TRIGGER event_redact_fill_reactions;
CREATE TRIGGER event_redact_fill_reactions
	AFTER UPDATE
	ON event
	WHEN NEW.type = 'm.reaction'
		AND NEW.relation_type = 'm.annotation'
		AND NEW.redacted_by IS NOT NULL
		AND OLD.redacted_by IS NULL
		AND typeof(NEW.content ->> '$."m.relates_to".key') = 'text'
BEGIN
	UPDATE event
	SET reactions=json_set(
		reactions,
		'$.' || json_quote(NEW.content ->> '$."m.relates_to".key'),
		coalesce(
			reactions ->> ('$.' || json_quote(NEW.content ->> '$."m.relates_to".key')),
			0
		) - 1)
	WHERE event_id = NEW.relates_to
	  AND room_id = NEW.room_id
	  AND reactions IS NOT NULL;

	UPDATE event
	SET own_reactions = json_set(
		own_reactions,
		'$.' || json_quote(NEW.content ->> '$."m.relates_to".key'),
		json((
			SELECT json_group_array(value)
			FROM json_each(own_reactions, '$.' || json_quote(NEW.content ->> '$."m.relates_to".key'))
			WHERE value <> NEW.event_id
		)))
	WHERE event_id = NEW.relates_to
	  AND room_id = NEW.room_id
	  AND NEW.sender IN (SELECT user_id FROM account)
	  AND own_reactions IS NOT NULL;
END;

-- Repair any values that were already stored as strings
UPDATE event
SET own_reactions = (
	SELECT json_group_object(key, json(value))
	FROM json_each(event.own_reactions)
)
WHERE own_reactions IS NOT NULL
  AND json_valid(own_reactions)
  AND EXISTS(SELECT 1 FROM json_each(event.own_reactions) WHERE type = 'text');
