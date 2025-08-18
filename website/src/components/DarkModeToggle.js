import React from 'react';

function DarkModeToggle({ isDark, onToggle }) {
  return (
    <button
      onClick={onToggle}
      className="relative inline-flex h-7 w-12 items-center rounded-full bg-slate-300 dark:bg-slate-600 transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:focus:ring-offset-slate-800"
      aria-label="Toggle dark mode"
    >
      {/* Toggle circle */}
      <span
        className={`inline-block h-5 w-5 transform rounded-full bg-white transition-transform duration-300 ease-in-out ${
          isDark ? 'translate-x-6' : 'translate-x-1'
        }`}
      />
      
      {/* Sun icon */}
      <span
        className={`absolute left-1 top-1 h-5 w-5 flex items-center justify-center transition-opacity duration-300 ${
          isDark ? 'opacity-0' : 'opacity-100'
        }`}
      >
        <i className="fas fa-sun text-xs text-amber-500"></i>
      </span>
      
      {/* Moon icon */}
      <span
        className={`absolute right-1 top-1 h-5 w-5 flex items-center justify-center transition-opacity duration-300 ${
          isDark ? 'opacity-100' : 'opacity-0'
        }`}
      >
        <i className="fas fa-moon text-xs text-slate-200"></i>
      </span>
    </button>
  );
}

export default DarkModeToggle;