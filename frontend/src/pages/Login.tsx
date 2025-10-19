import { Link } from 'react-router-dom';
import s from "./AuthPage.module.css";
import LoginForm from '../components/LoginForm'
import Logo from '../components/Logo';


function Login() {
  return (
    <div className={s.wrapper}>
      <div className={`${s.card} u-card u-card--lg u-stack`}>
        <div className={s.logoContainer}>
          <Logo size="medium" />
          <h1 className={s.brandName}>Interlink</h1>
        </div>
        <p className={s.subtitle}>Your analog dreams, digitally connected</p>

        <LoginForm />

        <p className={s.redirect}>
          Don&apos;t have an account? <Link to="/signup">Sign up</Link>
        </p>
      </div>
    </div>
  );
}

export default Login
