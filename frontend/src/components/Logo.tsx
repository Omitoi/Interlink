import React from 'react';
import logoImage from '../assets/logo.png';

interface LogoProps {
  size?: 'small' | 'medium' | 'large';
  className?: string;
}

const Logo: React.FC<LogoProps> = ({ 
  size = 'medium',
  className = ''
}) => {
  const sizeStyles = {
    small: { height: '32px', width: 'auto' },
    medium: { height: '48px', width: 'auto' },
    large: { height: '64px', width: 'auto' }
  };

  return (
    <img
      src={logoImage}
      alt="Interlink - Your analog dreams, digitally connected"
      style={sizeStyles[size]}
      className={`object-contain ${className}`}
    />
  );
};

export default Logo;
