//go:build cgo

// Обновление самой программы: поиск новой версии, скачивание и замена файла.
//
// Весь путь отличается от обновления чужих репозиториев: здесь нужен не
// архив с файлами, а готовый исполняемый файл, который заменит текущий.
package ui

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/model"
	"github.com/KosenkovPro/craftpull-go/internal/selfupdate"
)

// updateRepo возвращает репозиторий, из которого берутся обновления.
//
// По умолчанию — официальный репозиторий программы. Пользователь может
// указать другой через настройку self_update_repo.
func updateRepo(cfg config.Config) string {
	if cfg.SelfUpdateRepo != "" {
		return cfg.SelfUpdateRepo
	}
	return selfupdate.DefaultRepo
}

// checkSelfUpdate ищет новое обновление.
//
// quiet — тихая проверка при запуске: если обновлений нет, сообщение не
// показывается, в журнал пишется только результат.
func (u *UI) checkSelfUpdate(quiet bool) {
	if u.SelfBusy() {
		return
	}
	cfg := u.mgr.Config()
	repo := updateRepo(cfg)
	u.SetSelfBusy(true)
	u.mgr.Log(model.LogInfo, i18n.TF("log.update_check", map[string]string{"{r}": repo}))

	go func() {
		client := &http.Client{Timeout: selfupdate.Timeout}
		ctx, cancel := context.WithTimeout(context.Background(), selfupdate.Timeout)
		defer cancel()

		upd, err := selfupdate.Check(ctx, client, cfg.GitHubToken, repo, Version)
		u.SetSelfBusy(false)
		switch {
		case err != nil:
			u.mgr.Log(model.LogError, i18n.TF("log.update_err", map[string]string{"{e}": err.Error()}))
			if !quiet {
				u.st.SetNotice(i18n.TF("notify.update_failed", map[string]string{"{e}": err.Error()}))
			}
		case upd == nil:
			u.mgr.Log(model.LogOk, i18n.TF("log.update_none", map[string]string{"{v}": Version}))
			if !quiet {
				u.st.SetNotice(i18n.T("notify.update_none"))
			}
		default:
			u.SetSelfUpdate(upd)
			u.mgr.Log(model.LogOk, i18n.TF("log.update_found", map[string]string{"{v}": upd.Version}))
			u.st.SetNotice(i18n.TF("notify.update_available", map[string]string{"{v}": upd.Version}))
		}
	}()
}

// installSelfUpdate скачивает обновление, проверяет его и заменяет текущий
// файл программы на новый. После замены программа запускается заново.
func (u *UI) installSelfUpdate() {
	upd := u.SelfUpdate()
	if upd == nil {
		u.st.SetNotice(i18n.T("notify.update_none"))
		return
	}
	if u.SelfBusy() {
		return
	}
	cfg := u.mgr.Config()
	u.SetSelfBusy(true)
	u.mgr.Log(model.LogInfo, i18n.TF("log.update_download", map[string]string{"{v}": upd.Version}))
	u.st.SetNotice(i18n.TF("notify.update_downloading", map[string]string{"{v}": upd.Version}))

	go func() {
		client := &http.Client{Timeout: 15 * time.Minute}
		dir := selfupdate.UpdateDir(config.DataDir(), upd.Version)
		payload, err := selfupdate.Fetch(context.Background(), client, cfg.GitHubToken, upd, dir, nil)
		if err != nil {
			u.SetSelfBusy(false)
			u.mgr.Log(model.LogError, i18n.TF("log.update_err", map[string]string{"{e}": err.Error()}))
			u.st.SetNotice(i18n.TF("notify.update_failed", map[string]string{"{e}": err.Error()}))
			return
		}
		current, err := os.Executable()
		if err != nil {
			u.SetSelfBusy(false)
			u.mgr.Log(model.LogError, i18n.TF("log.update_err", map[string]string{"{e}": err.Error()}))
			u.st.SetNotice(i18n.TF("notify.update_failed", map[string]string{"{e}": err.Error()}))
			return
		}
		if err := selfupdate.Apply(current, payload); err != nil {
			u.SetSelfBusy(false)
			u.mgr.Log(model.LogError, i18n.TF("log.update_err", map[string]string{"{e}": err.Error()}))
			u.st.SetNotice(i18n.TF("notify.update_failed", map[string]string{"{e}": err.Error()}))
			return
		}
		u.mgr.Log(model.LogOk, i18n.TF("log.update_done", map[string]string{"{v}": upd.Version}))
		// Файл программы уже заменён — дальше работает новая копия.
		os.Exit(0)
	}()
}
