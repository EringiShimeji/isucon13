package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type LivestreamStatistics struct {
	Rank           int64 `json:"rank"`
	ViewersCount   int64 `json:"viewers_count"`
	TotalReactions int64 `json:"total_reactions"`
	TotalReports   int64 `json:"total_reports"`
	MaxTip         int64 `json:"max_tip"`
}

type LivestreamRankingEntry struct {
	LivestreamID int64
	Score        int64
}
type LivestreamRanking []LivestreamRankingEntry

func (r LivestreamRanking) Len() int      { return len(r) }
func (r LivestreamRanking) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r LivestreamRanking) Less(i, j int) bool {
	if r[i].Score == r[j].Score {
		return r[i].LivestreamID < r[j].LivestreamID
	} else {
		return r[i].Score < r[j].Score
	}
}

type UserStatistics struct {
	Rank              int64  `json:"rank"`
	ViewersCount      int64  `json:"viewers_count"`
	TotalReactions    int64  `json:"total_reactions"`
	TotalLivecomments int64  `json:"total_livecomments"`
	TotalTip          int64  `json:"total_tip"`
	FavoriteEmoji     string `json:"favorite_emoji"`
}

type UserRankingEntry struct {
	Username string
	Score    int64
}
type UserRanking []UserRankingEntry

func (r UserRanking) Len() int      { return len(r) }
func (r UserRanking) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r UserRanking) Less(i, j int) bool {
	if r[i].Score == r[j].Score {
		return r[i].Username < r[j].Username
	} else {
		return r[i].Score < r[j].Score
	}
}

func getUserRank(ctx context.Context, tx *sqlx.Tx, username string) (int64, error) {
	var rank int64 = 1
	if err := tx.GetContext(ctx, &rank, `
		WITH ranked AS (
			SELECT
				u.name AS name,
				ROW_NUMBER() OVER (ORDER BY COUNT(r.id) + IFNULL(SUM(lc.tip), 0) DESC, u.name DESC) AS rnk
			FROM users u
			LEFT JOIN livestreams l ON u.id = l.user_id
			LEFT JOIN reactions r ON l.id = r.livestream_id
			LEFT JOIN livecomments lc ON l.id = lc.livestream_id
			GROUP BY u.id
		)
		SELECT rnk
		FROM ranked
		WHERE name = ?
	`, username); err != nil {
		return 0, err
	}

	return rank, nil
}

func getUserStatisticsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	username := c.Param("username")
	// ユーザごとに、紐づく配信について、累計リアクション数、累計ライブコメント数、累計売上金額を算出
	// また、現在の合計視聴者数もだす

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	var user UserModel
	if err := tx.GetContext(ctx, &user, "SELECT * FROM users WHERE name = ?", username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusBadRequest, "not found user that has the given username")
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user: "+err.Error())
		}
	}

	// ランク算出
	rank, err := getUserRank(ctx, tx, username)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user rank: "+err.Error())
	}

	// リアクション数
	var totalReactions int64
	query := `SELECT COUNT(*) FROM users u 
    INNER JOIN livestreams l ON l.user_id = u.id 
    INNER JOIN reactions r ON r.livestream_id = l.id
    WHERE u.name = ?
	`
	if err := tx.GetContext(ctx, &totalReactions, query, username); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count total reactions: "+err.Error())
	}

	// ライブコメント数、チップ合計
	var totalLivecomments int64
	var totalTip int64
	var livestreams []*LivestreamModel
	if err := tx.SelectContext(ctx, &livestreams, "SELECT * FROM livestreams WHERE user_id = ?", user.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestreams: "+err.Error())
	}

	for _, livestream := range livestreams {
		var livecomments []*LivecommentModel
		if err := tx.SelectContext(ctx, &livecomments, "SELECT * FROM livecomments WHERE livestream_id = ?", livestream.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livecomments: "+err.Error())
		}

		for _, livecomment := range livecomments {
			totalTip += livecomment.Tip
			totalLivecomments++
		}
	}

	// 合計視聴者数
	var viewersCount int64
	for _, livestream := range livestreams {
		var cnt int64
		if err := tx.GetContext(ctx, &cnt, "SELECT COUNT(*) FROM livestream_viewers_history WHERE livestream_id = ?", livestream.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream_view_history: "+err.Error())
		}
		viewersCount += cnt
	}

	// お気に入り絵文字
	var favoriteEmoji string
	query = `
	SELECT r.emoji_name
	FROM users u
	INNER JOIN livestreams l ON l.user_id = u.id
	INNER JOIN reactions r ON r.livestream_id = l.id
	WHERE u.name = ?
	GROUP BY emoji_name
	ORDER BY COUNT(*) DESC, emoji_name DESC
	LIMIT 1
	`
	if err := tx.GetContext(ctx, &favoriteEmoji, query, username); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to find favorite emoji: "+err.Error())
	}

	stats := UserStatistics{
		Rank:              rank,
		ViewersCount:      viewersCount,
		TotalReactions:    totalReactions,
		TotalLivecomments: totalLivecomments,
		TotalTip:          totalTip,
		FavoriteEmoji:     favoriteEmoji,
	}
	return c.JSON(http.StatusOK, stats)
}

func getLivestreamRank(ctx context.Context, tx *sqlx.Tx, livestreamID int64) (int64, error) {
	var rank int64 = 1
	if err := tx.GetContext(ctx, &rank, `
		WITH ranked AS (
			SELECT
				l.id AS id,
				ROW_NUMBER() OVER (ORDER BY COUNT(r.id) + IFNULL(SUM(lc.tip), 0) DESC, id DESC) AS rnk
			FROM livestreams l
			LEFT JOIN reactions r ON l.id = r.livestream_id
			LEFT JOIN livecomments lc ON l.id = lc.livestream_id
			GROUP BY l.id
		)
		SELECT rnk
		FROM ranked
		WHERE id = ?
	`, livestreamID); err != nil {
		return 0, err
	}

	return rank, nil
}

func getLivestreamStatisticsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		return err
	}

	id, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}
	livestreamID := int64(id)

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	if _, ok := cache.getLivestreamModel(livestreamID); !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot get stats of not found livestream")
	}

	// ランク算出
	rank, err := getLivestreamRank(ctx, tx, livestreamID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream rank: "+err.Error())
	}

	// 視聴者数算出
	var viewersCount int64
	if err := tx.GetContext(ctx, &viewersCount, `SELECT COUNT(*) FROM livestreams l INNER JOIN livestream_viewers_history h ON h.livestream_id = l.id WHERE l.id = ?`, livestreamID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count livestream viewers: "+err.Error())
	}

	// 最大チップ額
	var maxTip int64
	if err := tx.GetContext(ctx, &maxTip, `SELECT IFNULL(MAX(tip), 0) FROM livestreams l INNER JOIN livecomments l2 ON l2.livestream_id = l.id WHERE l.id = ?`, livestreamID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to find maximum tip livecomment: "+err.Error())
	}

	// リアクション数
	var totalReactions int64
	if err := tx.GetContext(ctx, &totalReactions, "SELECT COUNT(*) FROM livestreams l INNER JOIN reactions r ON r.livestream_id = l.id WHERE l.id = ?", livestreamID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count total reactions: "+err.Error())
	}

	// スパム報告数
	var totalReports int64
	if err := tx.GetContext(ctx, &totalReports, `SELECT COUNT(*) FROM livestreams l INNER JOIN livecomment_reports r ON r.livestream_id = l.id WHERE l.id = ?`, livestreamID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count total spam reports: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, LivestreamStatistics{
		Rank:           rank,
		ViewersCount:   viewersCount,
		MaxTip:         maxTip,
		TotalReactions: totalReactions,
		TotalReports:   totalReports,
	})
}
