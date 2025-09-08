import { useState } from 'react';
import { Link } from 'react-router-dom';
import Logo from './Logo';
import DarkModeToggle from './DarkModeToggle';
import { useDarkMode } from '../contexts/DarkModeContext';

function Navbar() {
  const [isOpen, setIsOpen] = useState(false);
  const { isDark, toggleDarkMode } = useDarkMode();
  
  return (
    <nav className="bg-white dark:bg-slate-800 h-20 flex justify-center items-center sticky top-0 z-50 shadow-sm border-b border-slate-200 dark:border-slate-700">
      <div className="flex justify-between items-center w-full max-w-7xl px-5">
        <Logo />
        
        <div className="flex items-center gap-4 lg:order-2">
          <DarkModeToggle isDark={isDark} onToggle={toggleDarkMode} />
          <div className="lg:hidden text-blue-600 dark:text-blue-400 text-2xl cursor-pointer" onClick={() => setIsOpen(!isOpen)}>
            <i className={isOpen ? 'fas fa-times' : 'fas fa-bars'} />
          </div>
        </div>
        <ul className={`${isOpen ? 'flex' : 'hidden lg:flex'} flex-col lg:flex-row lg:items-center list-none lg:order-1 absolute lg:relative top-20 lg:top-0 left-0 lg:left-auto w-full lg:w-auto bg-white dark:bg-slate-800 lg:bg-transparent shadow-lg lg:shadow-none border-t lg:border-t-0 border-slate-200 dark:border-slate-700 py-4 lg:py-0 transition-all duration-300`}>
          <li className="mx-4 lg:mx-4">
            <Link 
              to="/" 
              className="flex items-center text-slate-700 dark:text-slate-300 hover:text-blue-600 dark:hover:text-blue-400 px-4 py-2 font-medium transition-colors" 
              onClick={() => setIsOpen(false)}
            >
              Inicio
            </Link>
          </li>
          <li className="mx-4 lg:mx-4">
            <Link 
              to="/content-manager" 
              className="flex items-center gap-2 text-slate-700 dark:text-slate-300 hover:text-blue-600 dark:hover:text-blue-400 px-4 py-2 font-medium transition-colors" 
              onClick={() => setIsOpen(false)}
            >
              <i className="fas fa-edit text-center flex items-center justify-center w-4"></i>
              <span>Gestión de Contenido</span>
            </Link>
          </li>
          <li className="mx-4 lg:mx-4">
            <Link 
              to="/login" 
              className="flex items-center gap-2 text-slate-700 dark:text-slate-300 hover:text-blue-600 dark:hover:text-blue-400 px-4 py-2 font-medium transition-colors" 
              onClick={() => setIsOpen(false)}
            >
              <i className="fas fa-link"></i>
              <span>Conectar Paginas</span>
            </Link>
          </li>
        </ul>
      </div>
    </nav>
  );
}

export default Navbar;