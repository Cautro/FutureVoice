import { ErrorBanner } from '../components/UI'

const errors: Record<string, string> = {
  telegram_login_failed: 'Telegram не завершил авторизацию. Попробуйте ещё раз.',
  invalid_login_state: 'Проверка авторизации не прошла. Повторите вход.',
  missing_authorization_code: 'Telegram не вернул код авторизации.',
  expired_login: 'Ссылка входа истекла. Начните авторизацию заново.',
  telegram_token_exchange_failed: 'Не удалось подтвердить вход через Telegram.',
  missing_id_token: 'Telegram не вернул подтверждение личности.',
  invalid_id_token: 'Подпись или данные токена Telegram не прошли проверку.',
  invalid_telegram_profile: 'Не удалось прочитать профиль Telegram.',
  invalid_login_nonce: 'Проверка nonce не прошла. Повторите вход.',
  account_not_allowed: 'У этого Telegram-аккаунта нет активной роли модератора или администратора.',
  session_creation_failed: 'Не удалось создать сессию. Попробуйте позже.',
}

export default function LoginPage() {
  const errorKey = new URLSearchParams(window.location.search).get('error')

  return (
    <main className="login-shell">
      <div className="login-card">
        <div className="brand brand-large">
          <span className="brand-mark">F</span>
          <span>FutureVoice</span>
        </div>

        <div className="login-eyebrow">Школьное самоуправление</div>
        <h1>Панель управления</h1>
        <p className="login-copy">
          Рабочее пространство для обработки обращений, управления доступом и аналитики.
        </p>

        {errorKey && (
          <ErrorBanner error={errors[errorKey] || 'Не удалось выполнить вход.'} />
        )}

        <a className="primary-button login-button" href="/api/v1/auth/telegram/start">
          <span className="telegram-mark">➤</span>
          Войти через Telegram
        </a>

        <div className="login-footnote">
          Доступ имеют только активные пользователи с ролью модератора или администратора.
        </div>
      </div>

      <div className="login-side-note">
        FUTUREVOICE <span>·</span> ADMIN WORKSPACE
      </div>
    </main>
  )
}