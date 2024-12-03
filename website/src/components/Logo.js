// Logo.js
import { Link } from 'react-router-dom';
import './Logo.css';

function Logo() {
  return (
    <Link to="/" className="logo">
      <img 
        src="/neurocrow_logo.png" 
        alt="Neurocrow" 
        className="logo-image"
      />
    </Link>
  );
}

export default Logo;