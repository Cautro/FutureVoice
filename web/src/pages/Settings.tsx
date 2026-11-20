import { GRAFANA_URL } from '../utils/constants'
import { Topbar } from '../components/UI'

export default function SettingsPage({ user }: { user?: { role: string } }) {
  return (
    <div className="page-content">
      <Topbar title="Инструменты" description="Ссылки и информация о наблюдаемости FutureVoice." />

      <div className="settings-grid">
        {user?.role === 'admin' && (
          <section className="panel settings-card">
            <div className="settings-icon purple">⌁</div>
            <h2>Grafana</h2>
            <p>
              Технический мониторинг backend: частота HTTP-запросов, ошибки, задержки, Go runtime и
              пул PostgreSQL.
            </p>
            <a
              className="secondary-button link-button"
              href={GRAFANA_URL}
              target="_blank"
              rel="noreferrer"
            >
              Открыть Grafana ↗
            </a>
            <small>Локальный адрес по умолчанию. На сервере используйте настроенный домен.</small>
          </section>
        )}

        <section className="panel settings-card">
          <div className="settings-icon green">▥</div>
          <h2>Бизнес-аналитика</h2>
          <p>Динамика обращений, распределение по типам и статусам, среднее время закрытия.</p>
          <span className="settings-tag">В разделе «Обзор»</span>
          <small>
            Экспорт PNG формируется в браузере из агрегированных данных без персональных данных.
          </small>
        </section>

        <section className="panel settings-card">
          <div className="settings-icon blue">♙</div>
          <h2>Доступ</h2>
          <p>
            Вход через Telegram OIDC. Серверная сессия хранится в PostgreSQL, cookie защищена
            HttpOnly.
          </p>
          <span className="settings-tag">Роли проверяются на backend</span>
          <small>В production обязательно включите HTTPS и COOKIE_SECURE=true.</small>
        </section>
      </div>
    </div>
  )
}