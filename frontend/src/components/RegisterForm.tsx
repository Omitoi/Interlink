import { useState } from 'react'
import axios from '../api/axios'
import { isAxiosError } from 'axios'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import s from "./Form.module.css";

function RegisterForm() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const { refreshAuth } = useAuth()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    setLoading(true)

    try {
      // Register the user (which now returns a token)
      const response = await axios.post<{ token: string; id: number }>('/register', { email, password })
      
      // Store the token in localStorage
      localStorage.setItem('token', response.data.token)
      
      // Refresh auth context to pick up the new token immediately
      await refreshAuth()
      
      setSuccess('Registration successful! Redirecting...')
      
      // Navigate to profile wizard for new users
      setTimeout(() => {
        navigate('/profile/wizard')
      }, 1000)
    } catch (err: Error | unknown) {
      // Surface clearer error reasons and network failures
      if (isAxiosError(err)) {
        type ErrorBody = { error?: string }
        const status = err.response?.status
        const code = (err.response?.data as ErrorBody | undefined)?.error
        if (status === 409 || code === 'email_exists') {
          setError('Email already registered.')
          return
        }
        if (err.code === 'ERR_NETWORK') {
          console.error('Registration network error:', err)
          setError('Network error contacting the server. Is the backend running?')
          return
        }
        console.error('Registration error:', { status, code, err })
        setError(`Registration failed${status ? ` (${status}${code ? ` - ${code}` : ''})` : ''}.`)
      } else {
        console.error('Registration unexpected error:', err)
        setError('Registration failed.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="u-stack">
      <h2>Register</h2>

      {error && <div className={s.error}>{error}</div>}
      {success && <div className={s.success}>{success}</div>}

      <input
        className={s.input}
        type="email"
        placeholder="Email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
      />
      <input
        className={s.input}
        type="password"
        placeholder="Password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
      />

      <button type="submit" className="u-btn u-btn--primary" disabled={loading}>
        {loading ? 'Registering...' : 'Sign up'}
      </button>
    </form>
  )
}

export default RegisterForm
