package post_sync_migrations

import (
	"context"
	"github.com/uptrace/bun"
)

// TODO: revisit access group relationships when we refactor the messaging app to use the graphql API.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if !calculateExplorerStatistics {
			return nil
		}

		err := RunMigrationWithRetries(db, `
			CREATE TABLE public_key_first_transaction (
				public_key VARCHAR PRIMARY KEY ,
				timestamp TIMESTAMP,
				height BIGINT
			);
			
			CREATE INDEX idx_public_key_first_transaction_timestamp
			ON public_key_first_transaction (timestamp desc);
			
			CREATE INDEX idx_public_key_first_transaction_height
			ON public_key_first_transaction (height desc);
			
			INSERT INTO public_key_first_transaction (public_key, timestamp, height)
			select apk.public_key, min(b.timestamp), min(b.height) FROM affected_public_key apk
			JOIN transaction t ON apk.transaction_hash = t.transaction_hash
			JOIN block b ON t.block_hash = b.block_hash
			group by apk.public_key;
		`)
		if err != nil {
			return err
		}

		err = RunMigrationWithRetries(db, `
			CREATE OR REPLACE FUNCTION refresh_public_key_first_transaction()
			RETURNS VOID AS $$
			DECLARE
				max_timestamp TIMESTAMP;
			BEGIN
				-- Get the maximum timestamp currently in the table
				SELECT MAX(timestamp) INTO max_timestamp
				FROM public_key_first_transaction;
			
				-- Insert new rows for public keys that are not in the table yet
				INSERT INTO public_key_first_transaction (public_key, timestamp, height)
				SELECT apk.public_key, min(b.timestamp), min(b.height) FROM affected_public_key apk
				JOIN transaction t ON apk.transaction_hash = t.transaction_hash
				JOIN block b ON t.block_hash = b.block_hash
				WHERE b.timestamp > max_timestamp
				group by apk.public_key
				ON CONFLICT (public_key) DO NOTHING;
			END;
			$$ LANGUAGE plpgsql
		`)
		if err != nil {
			return err
		}

		err = RunMigrationWithRetries(db, `
			CREATE OR REPLACE FUNCTION get_transaction_count(transaction_type integer)
			RETURNS bigint AS
			$BODY$
			DECLARE
				count_value bigint;
				padded_transaction_type varchar;
			BEGIN
				IF transaction_type < 1 OR transaction_type > 33 THEN
					RAISE EXCEPTION '% is not a valid transaction type', transaction_type;
				END IF;
			
				padded_transaction_type := LPAD(transaction_type::text, 2, '0');
			
				EXECUTE format('SELECT COALESCE(reltuples::bigint, 0) FROM pg_class WHERE relname = ''transaction_partition_%s''', padded_transaction_type) INTO count_value;
				RETURN count_value;
			END;
			$BODY$
			LANGUAGE plpgsql
		`)
		if err != nil {
			return err
		}

		err = RunMigrationWithRetries(db, `
			CREATE MATERIALIZED VIEW statistic_txn_count_all AS
			SELECT SUM(get_transaction_count(s.i)) as count,
			       0 as id
			FROM generate_series(1, 33) AS s(i);

            CREATE UNIQUE INDEX statistic_txn_count_all_unique_index ON statistic_txn_count_all (id);

			CREATE MATERIALIZED VIEW statistic_txn_count_30_d AS
			select count(*), 0 as id from transaction t
			join block b
			on t.block_hash = b.block_hash
			where b.timestamp > NOW() - INTERVAL '30 days';

            CREATE UNIQUE INDEX statistic_txn_count_30_d_unique_index ON statistic_txn_count_30_d (id);

			CREATE MATERIALIZED VIEW statistic_block_height_current AS
			select height, 0 as id from block order by height desc limit 1;

            CREATE UNIQUE INDEX statistic_block_height_current_unique_index ON statistic_block_height_current (id);

			CREATE MATERIALIZED VIEW statistic_txn_count_pending AS
			select count(*) as count, 0 as id from transaction where block_hash = '';

            CREATE UNIQUE INDEX statistic_txn_count_pending_unique_index ON statistic_txn_count_pending (id);

			CREATE MATERIALIZED VIEW statistic_txn_fee_1_d AS
			select avg(t.fee_nanos) as avg, 0 as id from transaction_partition_05 t
			join block b on t.block_hash = b.block_hash
			where b.timestamp > NOW() - INTERVAL '1 day'
			and t.fee_nanos != 0;

            CREATE UNIQUE INDEX statistic_txn_fee_1_d_unique_index ON statistic_txn_fee_1_d (id);

			CREATE MATERIALIZED VIEW statistic_total_supply AS
			select sum(balance_nanos) as sum, 0 as id from deso_balance_entry;

            CREATE UNIQUE INDEX statistic_total_supply_unique_index ON statistic_total_supply (id);

			CREATE MATERIALIZED VIEW statistic_post_count AS
			select count(post_hash) as count, 0 as id from post_entry
			where parent_post_hash is null
			and reposted_post_hash is null
			and NOT (post_entry.extra_data ? 'BlogDeltaRtfFormat');

            CREATE UNIQUE INDEX statistic_post_count_unique_index ON statistic_post_count (id);

			CREATE MATERIALIZED VIEW statistic_post_longform_count AS
			select count(post_hash) as count, 0 as id from post_entry
			where parent_post_hash is null
			and reposted_post_hash is null
			and (post_entry.extra_data ? 'BlogDeltaRtfFormat');

            CREATE UNIQUE INDEX statistic_post_longform_count_unique_index ON statistic_post_longform_count (id);

			CREATE MATERIALIZED VIEW statistic_comment_count AS
			select count(post_hash), 0 as id from post_entry
			where parent_post_hash is not null;

            CREATE UNIQUE INDEX statistic_comment_count_unique_index ON statistic_comment_count (id);

			CREATE MATERIALIZED VIEW statistic_repost_count AS
			select count(post_hash), 0 as id from post_entry
			where reposted_post_hash is not null;

            CREATE UNIQUE INDEX statistic_repost_count_unique_index ON statistic_repost_count (id);


			CREATE MATERIALIZED VIEW statistic_txn_count_creator_coin AS
			select get_transaction_count(11) +
				   get_transaction_count(14) as count, 0 as id;

            CREATE UNIQUE INDEX statistic_txn_count_creator_coin_unique_index ON statistic_txn_count_creator_coin (id);

			CREATE MATERIALIZED VIEW statistic_txn_count_nft AS
			select get_transaction_count(15) +
				   get_transaction_count(16) +
				   get_transaction_count(17) +
				   get_transaction_count(18) +
				   get_transaction_count(19) +
				   get_transaction_count(20) +
				   get_transaction_count(21) as count, 0 as id;

            CREATE UNIQUE INDEX statistic_txn_count_nft_unique_index ON statistic_txn_count_nft (id);

			CREATE MATERIALIZED VIEW statistic_txn_count_dex AS
			select get_transaction_count(24) +
				   get_transaction_count(25) +
				   get_transaction_count(26) as count, 0 as id;

            CREATE UNIQUE INDEX statistic_txn_count_dex_unique_index ON statistic_txn_count_dex (id);

			CREATE MATERIALIZED VIEW statistic_txn_count_social AS
			select get_transaction_count(4) +
				   get_transaction_count(5) +
				   get_transaction_count(6) +
				   get_transaction_count(9) +
				   get_transaction_count(10) +
				   get_transaction_count(23) +
				   get_transaction_count(27) +
				   get_transaction_count(28) +
				   get_transaction_count(29) +
				   get_transaction_count(30) +
				   get_transaction_count(31) +
				   get_transaction_count(32) +
				   get_transaction_count(33) as count, 0 as id;

            CREATE UNIQUE INDEX statistic_txn_count_social_unique_index ON statistic_txn_count_social (id);

			CREATE MATERIALIZED VIEW statistic_follow_count AS
			SELECT reltuples::bigint AS count, 0 as id
			FROM pg_class
			WHERE relname = 'follow_entry';

            CREATE UNIQUE INDEX statistic_follow_count_unique_index ON statistic_follow_count (id);

			CREATE MATERIALIZED VIEW statistic_message_count AS
			SELECT SUM(count) as count, 0 as id
			FROM (
			SELECT reltuples::bigint AS count
			FROM pg_class
			WHERE relname = 'message_entry'
			UNION ALL
			SELECT reltuples::bigint AS count
			FROM pg_class
			WHERE relname = 'new_message_entry'
			) AS subquery;

            CREATE UNIQUE INDEX statistic_message_count_unique_index ON statistic_message_count (id);

			CREATE MATERIALIZED VIEW statistic_wallet_count_all AS
			SELECT COALESCE(reltuples::bigint, 0) as count, 0 as id FROM pg_class WHERE relname = 'public_key_first_transaction';

            CREATE UNIQUE INDEX statistic_wallet_count_all_unique_index ON statistic_wallet_count_all (id);

			CREATE MATERIALIZED VIEW statistic_new_wallet_count_30_d AS
			SELECT count(*), 0 as id from public_key_first_transaction
			WHERE timestamp > NOW() - INTERVAL '30 days';

            CREATE UNIQUE INDEX statistic_new_wallet_count_30_d_unique_index ON statistic_new_wallet_count_30_d (id);

			CREATE MATERIALIZED VIEW statistic_active_wallet_count_30_d AS
			WITH filtered_block AS (
			  SELECT block_hash
			  FROM block
			  WHERE timestamp > current_date - interval '1 month'
			)
			SELECT COUNT(DISTINCT t.public_key), 0 as id
			FROM transaction_partitioned t
			JOIN filtered_block fb ON t.block_hash = fb.block_hash;

            CREATE UNIQUE INDEX statistic_active_wallet_count_30_d_unique_index ON statistic_active_wallet_count_30_d (id);

			CREATE MATERIALIZED VIEW statistic_social_leaderboard_likes AS
			select count(*) as count, pe.poster_public_key, row_number() OVER () AS id from transaction_partition_10 t
			join post_entry pe on t.tx_index_metadata ->> 'PostHashHex' = pe.post_hash
			join block b on t.block_hash = b.block_hash
			where b.timestamp > NOW() - INTERVAL '30 days'
			and t.tx_index_metadata ->> 'IsUnlike' = 'false'
			group by pe.poster_public_key;

            CREATE UNIQUE INDEX statistic_social_leaderboard_likes_unique_index ON statistic_social_leaderboard_likes (id);

			CREATE MATERIALIZED VIEW statistic_social_leaderboard_reactions AS
			select count(*) as count, pe.poster_public_key, row_number() OVER () AS id from transaction_partition_29 t
			join post_entry pe on t.tx_index_metadata ->> 'PostHashHex' = pe.post_hash
			join block b on t.block_hash = b.block_hash
			where b.timestamp > NOW() - INTERVAL '30 days'
			and t.tx_index_metadata ->> 'AssociationType' = 'REACTION'
			group by pe.poster_public_key;

            CREATE UNIQUE INDEX statistic_social_leaderboard_reactions_unique_index ON statistic_social_leaderboard_reactions (id);

			CREATE MATERIALIZED VIEW statistic_social_leaderboard_diamonds AS
			select count(*), pe.poster_public_key, row_number() OVER () AS id from transaction_partition_02 t
			join post_entry pe on t.tx_index_metadata ->> 'PostHashHex' = pe.post_hash
			join block b on t.block_hash = b.block_hash
			where b.timestamp > NOW() - INTERVAL '30 days'
			group by pe.poster_public_key;

            CREATE UNIQUE INDEX statistic_social_leaderboard_diamonds_unique_index ON statistic_social_leaderboard_diamonds (id);

			CREATE MATERIALIZED VIEW statistic_social_leaderboard_reposts AS
			select count(*), pe.poster_public_key, row_number() OVER () AS id from post_entry pe
			join post_entry per on per.reposted_post_hash = pe.post_hash
			where per.timestamp > NOW() - INTERVAL '30 days'
			and pe.timestamp > NOW() - INTERVAL '30 days'
			group by pe.poster_public_key;

            CREATE UNIQUE INDEX statistic_social_leaderboard_reposts_unique_index ON statistic_social_leaderboard_reposts (id);

			CREATE MATERIALIZED VIEW statistic_social_leaderboard_comments AS
			select count(*), pe.poster_public_key, row_number() OVER () AS id from post_entry pe
			join post_entry pec on pec.parent_post_hash = pe.post_hash
			where pec.timestamp > NOW() - INTERVAL '30 days'
			and pe.timestamp > NOW() - INTERVAL '30 days'
			group by pe.poster_public_key;

            CREATE UNIQUE INDEX statistic_social_leaderboard_comments_unique_index ON statistic_social_leaderboard_comments (id);

			CREATE MATERIALIZED VIEW statistic_social_leaderboard AS
			select social_leaderboard.count, pe.*, row_number() OVER () AS id from (
				select sum(social_interactions.count) as count, social_interactions.poster_public_key from (
					select count, poster_public_key from statistic_social_leaderboard_likes

					UNION ALL

					select count, poster_public_key from statistic_social_leaderboard_reactions

					UNION ALL

					select count, poster_public_key from statistic_social_leaderboard_diamonds

					UNION ALL

					select count, poster_public_key from statistic_social_leaderboard_reposts

					UNION ALL

					select count, poster_public_key from statistic_social_leaderboard_comments

				) as social_interactions
				group by poster_public_key
				order by sum(count) desc
				limit 10
			) as social_leaderboard
			join profile_entry pe
			on social_leaderboard.poster_public_key = pe.public_key
			order by social_leaderboard.count desc;

            CREATE UNIQUE INDEX statistic_social_leaderboard_unique_index ON statistic_social_leaderboard (id);

			CREATE MATERIALIZED VIEW statistic_nft_leaderboard AS
			select sum(COALESCE(CAST(tx_index_metadata ->> 'BidAmountNanos' AS BIGINT), 0)), t.public_key, pe.username, row_number() OVER () AS id from transaction_partition_17 t
			join nft_entry ne
				on tx_index_metadata ->> 'NFTPostHashHex' = ne.nft_post_hash
				and tx_index_metadata ->> 'SerialNumber' = text(ne.serial_number)
			join block b
			on b.block_hash = t.block_hash
			left join profile_entry pe on t.public_key = pe.public_key
			where b.timestamp > NOW() - INTERVAL '30 days'
			group by t.public_key, pe.username
			order by sum(COALESCE(CAST(tx_index_metadata ->> 'BidAmountNanos' AS BIGINT), 0)) desc
			limit 10;

            CREATE UNIQUE INDEX statistic_nft_leaderboard_unique_index ON statistic_nft_leaderboard (id);

			create or replace function hex_to_decimal(hexval character varying) returns numeric
				language plpgsql
			as
			$$
			DECLARE
				result  numeric;
			BEGIN
			  EXECUTE 'SELECT x''' || hexval || '''::bit(64)::bigint' INTO result;
			  RETURN result;
			END;
			$$;

			CREATE MATERIALIZED VIEW statistic_defi_leaderboard AS
			select top_tokens.*, pe.*, row_number() OVER () AS id from (
				WITH buying AS (
					SELECT
						value ->> 'BuyingDAOCoinCreatorPublicKey' AS buying_public_key,
						SUM(hex_to_decimal(substring((value ->> 'CoinQuantityInBaseUnitsSold') from 3))) as quantity_sold
					FROM
						transaction_partition_26 t
					INNER JOIN
						block b
					ON
						t.block_hash = b.block_hash
					, jsonb_array_elements(t.tx_index_metadata->'FilledDAOCoinLimitOrdersMetadata') as value
					WHERE
						value ->> 'SellingDAOCoinCreatorPublicKey' = 'BC1YLbnP7rndL92x7DbLp6bkUpCgKmgoHgz7xEbwhgHTps3ZrXA6LtQ'
					AND
						b.timestamp > (NOW() - INTERVAL '30 days')
					GROUP BY
						buying_public_key
				), selling AS (
					SELECT
						value ->> 'SellingDAOCoinCreatorPublicKey' AS selling_public_key,
						SUM(hex_to_decimal(substring((value ->> 'CoinQuantityInBaseUnitsSold') from 3))) as quantity_sold
					FROM
						transaction_partition_26 t
					INNER JOIN
						block b
					ON
						t.block_hash = b.block_hash
					, jsonb_array_elements(t.tx_index_metadata->'FilledDAOCoinLimitOrdersMetadata') as value
					WHERE
						value ->> 'BuyingDAOCoinCreatorPublicKey' = 'BC1YLbnP7rndL92x7DbLp6bkUpCgKmgoHgz7xEbwhgHTps3ZrXA6LtQ'
					AND
						b.timestamp > (NOW() - INTERVAL '30 days')
					GROUP BY
						selling_public_key
				)
				SELECT
					buying.buying_public_key,
					(buying.quantity_sold - COALESCE(selling.quantity_sold, 0)) AS net_quantity
				FROM
					buying
				LEFT JOIN
					selling
				ON
					buying.buying_public_key = selling.selling_public_key
			) top_tokens
			join profile_entry pe on top_tokens.buying_public_key = pe.public_key
			order by top_tokens.net_quantity desc
			limit 10;

            CREATE UNIQUE INDEX statistic_defi_leaderboard_unique_index ON statistic_defi_leaderboard (id);

			CREATE MATERIALIZED VIEW statistic_txn_count_monthly AS
			SELECT date_trunc('month', b.timestamp) AS month, COUNT(*) AS transaction_count, row_number() OVER () AS id
			FROM transaction t
			JOIN block b ON t.block_hash = b.block_hash
			WHERE b.timestamp > NOW() - INTERVAL '1 year'
			GROUP BY month;

            CREATE UNIQUE INDEX statistic_txn_count_monthly_unique_index ON statistic_txn_count_monthly (id);

			CREATE MATERIALIZED VIEW statistic_wallet_count_monthly AS
			SELECT date_trunc('month', timestamp) AS month, COUNT(*) AS wallet_count, row_number() OVER () AS id
			FROM public_key_first_transaction
			WHERE timestamp > NOW() - INTERVAL '1 year'
			GROUP BY month;

            CREATE UNIQUE INDEX statistic_wallet_count_monthly_unique_index ON statistic_wallet_count_monthly (id);

			CREATE MATERIALIZED VIEW statistic_txn_count_daily AS
			SELECT DATE(b.timestamp) AS day, COUNT(*) AS transaction_count, row_number() OVER () AS id
			FROM transaction t
			JOIN block b ON t.block_hash = b.block_hash
			WHERE b.timestamp > NOW() - INTERVAL '1 month'
			GROUP BY day;

            CREATE UNIQUE INDEX statistic_txn_count_daily_unique_index ON statistic_txn_count_daily (id);

			CREATE MATERIALIZED VIEW statistic_new_wallet_count_daily AS
			SELECT date(timestamp) AS day, COUNT(*) AS wallet_count, row_number() OVER () AS id
			FROM public_key_first_transaction
			WHERE timestamp > NOW() - INTERVAL '1 month'
			GROUP BY day;

            CREATE UNIQUE INDEX statistic_new_wallet_count_daily_unique_index ON statistic_new_wallet_count_daily (id);

			CREATE MATERIALIZED VIEW statistic_active_wallet_count_daily AS
			WITH filtered_block AS (
			  SELECT block_hash, DATE(timestamp) as day
			  FROM block
			  WHERE timestamp > current_date - interval '1 month'
			)
			SELECT fb.day, COUNT(DISTINCT t.public_key), row_number() OVER () AS id
			FROM transaction_partitioned t
			JOIN filtered_block fb ON t.block_hash = fb.block_hash
			GROUP BY fb.day
			ORDER BY fb.day;

            CREATE UNIQUE INDEX statistic_active_wallet_count_daily_unique_index ON statistic_active_wallet_count_daily (id);
		`)
		if err != nil {
			return err
		}

		err = RunMigrationWithRetries(db, `
			CREATE VIEW statistic_dashboard AS
			SELECT
				statistic_txn_count_all.count as txn_count_all,
				statistic_txn_count_30_d.count as txn_count_30_d,
				statistic_wallet_count_all.count as wallet_count_all,
				statistic_active_wallet_count_30_d.count as active_wallet_count_30_d,
				statistic_new_wallet_count_30_d.count as new_wallet_count_30_d,
				statistic_block_height_current.height as block_height_current,
				statistic_txn_count_pending.count as txn_count_pending,
				statistic_txn_fee_1_d.avg as txn_fee_1_d,
				statistic_total_supply.sum as total_supply,
				statistic_post_count.count as post_count,
				statistic_post_longform_count.count as post_longform_count,
				statistic_comment_count.count as comment_count,
				statistic_repost_count.count as repost_count,
				statistic_txn_count_creator_coin.count as txn_count_creator_coin,
				statistic_txn_count_nft.count as txn_count_nft,
				statistic_txn_count_dex.count as txn_count_dex,
				statistic_txn_count_social.count as txn_count_social,
				statistic_follow_count.count as follow_count,
				statistic_message_count.count as message_count
			FROM
			statistic_txn_count_all
			CROSS JOIN
			statistic_txn_count_30_d
			CROSS JOIN
			statistic_wallet_count_all
			CROSS JOIN
			statistic_active_wallet_count_30_d
			CROSS JOIN
			statistic_new_wallet_count_30_d
			CROSS JOIN
			statistic_block_height_current
			CROSS JOIN
			statistic_txn_count_pending
			CROSS JOIN
			statistic_txn_fee_1_d
			CROSS JOIN
			statistic_total_supply
			CROSS JOIN
			statistic_post_count
			CROSS JOIN
			statistic_post_longform_count
			CROSS JOIN
			statistic_comment_count
			CROSS JOIN
			statistic_repost_count
			CROSS JOIN
			statistic_txn_count_creator_coin
			CROSS JOIN
			statistic_txn_count_nft
			CROSS JOIN
			statistic_txn_count_dex
			CROSS JOIN
			statistic_txn_count_social
			CROSS JOIN
			statistic_follow_count
			CROSS JOIN
			statistic_message_count;
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		if !calculateExplorerStatistics {
			return nil
		}
		_, err := db.Exec(`
			DROP FUNCTION IF EXISTS refresh_statistic_views;
			DROP VIEW IF EXISTS statistic_dashboard;
			DROP MATERIALIZED VIEW IF EXISTS statistic_txn_count_all;
			DROP MATERIALIZED VIEW IF EXISTS statistic_txn_count_30_d;
			DROP MATERIALIZED VIEW IF EXISTS statistic_wallet_count_all;
			DROP MATERIALIZED VIEW IF EXISTS statistic_new_wallet_count_30_d;
			DROP MATERIALIZED VIEW IF EXISTS statistic_active_wallet_count_30_d;
			DROP MATERIALIZED VIEW IF EXISTS statistic_block_height_current;
			DROP MATERIALIZED VIEW IF EXISTS statistic_txn_count_pending;
			DROP MATERIALIZED VIEW IF EXISTS statistic_txn_fee_1_d;
			DROP MATERIALIZED VIEW IF EXISTS statistic_total_supply;
			DROP MATERIALIZED VIEW IF EXISTS statistic_post_count;
			DROP MATERIALIZED VIEW IF EXISTS statistic_comment_count;
			DROP MATERIALIZED VIEW IF EXISTS statistic_repost_count;
			DROP MATERIALIZED VIEW IF EXISTS statistic_txn_count_creator_coin;
			DROP MATERIALIZED VIEW IF EXISTS statistic_txn_count_nft;
			DROP MATERIALIZED VIEW IF EXISTS statistic_txn_count_dex;
			DROP MATERIALIZED VIEW IF EXISTS statistic_txn_count_social;
			DROP MATERIALIZED VIEW IF EXISTS statistic_follow_count;
			DROP MATERIALIZED VIEW IF EXISTS statistic_message_count;
			DROP MATERIALIZED VIEW IF EXISTS statistic_social_leaderboard_likes;
			DROP MATERIALIZED VIEW IF EXISTS statistic_social_leaderboard_reactions;
			DROP MATERIALIZED VIEW IF EXISTS statistic_social_leaderboard_diamonds;
			DROP MATERIALIZED VIEW IF EXISTS statistic_social_leaderboard_reposts;
			DROP MATERIALIZED VIEW IF EXISTS statistic_social_leaderboard_comments;
			DROP MATERIALIZED VIEW IF EXISTS statistic_social_leaderboard;
			DROP MATERIALIZED VIEW IF EXISTS statistic_nft_leaderboard;
			DROP MATERIALIZED VIEW IF EXISTS statistic_defi_leaderboard;
			DROP MATERIALIZED VIEW IF EXISTS statistic_txn_count_monthly;
			DROP MATERIALIZED VIEW IF EXISTS statistic_wallet_count_monthly;
			DROP MATERIALIZED VIEW IF EXISTS statistic_txn_count_daily;
			DROP MATERIALIZED VIEW IF EXISTS statistic_new_wallet_count_daily;
			DROP MATERIALIZED VIEW IF EXISTS statistic_active_wallet_count_daily;
			DROP TABLE IF EXISTS public_key_first_transaction;
			DROP FUNCTION IF EXISTS refresh_public_key_first_transaction;
			DROP FUNCTION IF EXISTS get_transaction_count;
			DROP FUNCTION IF EXISTS hex_to_decimal;
		`)
		if err != nil {
			return err
		}
		return nil
	})
}
