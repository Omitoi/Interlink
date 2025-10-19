import { describe, it, expect } from 'vitest'

describe('Basic Tests', () => {
  it('should pass a simple test', () => {
    expect(1 + 1).toBe(2)
  })

  it('DOM should be available', () => {
    const div = document.createElement('div')
    div.textContent = 'test'
    expect(div.textContent).toBe('test')
  })
})