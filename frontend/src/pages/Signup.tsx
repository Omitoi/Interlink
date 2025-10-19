import RegisterForm from '../components/RegisterForm'
import s from "./AuthPage.module.css";
import { Link } from 'react-router-dom'
import Logo from '../components/Logo'

function Signup() {
  return (
    <div className={s.wrapper}>
      <div className={`${s.card} u-card u-card--lg u-stack`}>
        <div className={s.logoContainer}>
          <Logo size="medium" />
          <h1 className={s.brandName}>Interlink</h1>
        </div>
        <p className={s.subtitle}>Your analog dreams, digitally connected</p>

        <RegisterForm />

        <p className={s.redirect}>
          Already have an account? <Link to="/">Log in</Link>
        </p>
      </div>
    </div>
  )
}

export default Signup
