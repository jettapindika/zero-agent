// Sign-in screen shown when /auth/me returns 401. Centered card with the
// official Google "Sign in with Google" button styling. The button hands off
// to the system browser via the Tauri shell plugin; the daemon's
// /auth/google/callback closes the loop and the desktop refetches /auth/me.

type Props = {
  onSignIn: () => Promise<void> | void;
  reason?: string;
};

export function LoginView({ onSignIn, reason }: Props) {
  return (
    <div className="login-shell">
      <main className="login-card" role="dialog" aria-labelledby="login-title">
        <h1 id="login-title" className="login-title">Sign in to Zero</h1>
        <p className="login-sub">
          Local-first AI coding agent. Your password never reaches Zero — we use
          OpenID Connect via Google.
        </p>

        <button
          aria-label="Sign in with Google"
          className="google-button"
          onClick={() => {
            void onSignIn();
          }}
          type="button"
        >
          <GoogleGlyph />
          <span>Sign in with Google</span>
        </button>

        {reason ? <p className="login-error">Daemon reported: {reason}</p> : null}

        <p className="login-foot">
          By continuing you agree the daemon may store your Google profile id,
          email, name, and avatar in <code>~/.zero/zero.db</code>.
        </p>
      </main>
    </div>
  );
}

// Inline SVG of the Google G mark, per Google's branding guide. No external
// asset needed.
function GoogleGlyph() {
  return (
    <svg aria-hidden="true" focusable="false" height="18" viewBox="0 0 18 18" width="18">
      <path
        d="M17.64 9.2045c0-.6381-.0573-1.2518-.1636-1.8409H9v3.4814h4.8436c-.2086 1.125-.8427 2.0782-1.7959 2.7164v2.2581h2.9081C16.6582 14.2527 17.64 11.9445 17.64 9.2045z"
        fill="#4285F4"
      />
      <path
        d="M9 18c2.43 0 4.4673-.806 5.9559-2.1818l-2.9081-2.2581c-.806.54-1.8368.8591-3.0477.8591-2.3445 0-4.3286-1.5832-5.0364-3.71H.9573v2.3318C2.4382 15.9832 5.4818 18 9 18z"
        fill="#34A853"
      />
      <path
        d="M3.9636 10.71c-.18-.54-.2823-1.1168-.2823-1.71s.1023-1.17.2823-1.71V4.9582H.9573C.3477 6.1727 0 7.5477 0 9s.3477 2.8273.9573 4.0418l3.0063-2.3318z"
        fill="#FBBC05"
      />
      <path
        d="M9 3.5795c1.3214 0 2.5077.4545 3.4405 1.346l2.5814-2.5814C13.4632.8918 11.4259 0 9 0 5.4818 0 2.4382 2.0168.9573 4.9582l3.0063 2.3318C4.6714 5.1627 6.6555 3.5795 9 3.5795z"
        fill="#EA4335"
      />
    </svg>
  );
}
