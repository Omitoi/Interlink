import logoImage from '../assets/logo.png';

const NotFound = () => {
  return (
    <div className="u-fullheight u-flex-center" style={{ padding: 'var(--space-6)', background: 'var(--color-bg)' }}>
      <div className="u-card u-card--lg u-stack" style={{ textAlign: 'center', maxWidth: '400px' }}>
        <img 
          src={logoImage} 
          alt="Interlink Logo" 
          style={{
            maxWidth: '200px',
            height: 'auto',
            margin: '0 auto var(--space-5) auto'
          }}
        />
        <p style={{
          fontSize: '18px',
          color: 'var(--color-text-muted)',
          margin: 0
        }}>
          Seems like we've wandered too far off
        </p>
      </div>
    </div>
  );
};

export default NotFound;
