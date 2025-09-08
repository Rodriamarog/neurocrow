// Logo.js
import { Link } from 'react-router-dom';

function Logo() {
  return (
    <Link to="/" className="flex items-center no-underline py-0">
      <img 
        src="/neurocrow-logo.png" 
        alt="Neurocrow" 
        className="h-20 w-auto object-contain transition-all duration-300 dark:brightness-0 dark:invert"
      />
    </Link>
  );
}

export default Logo;