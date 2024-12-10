import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import './Login.css';

function Login() {
  const navigate = useNavigate();
  const [isVerifying, setIsVerifying] = useState(false);
  const [pollInterval, setPollInterval] = useState(null);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollInterval) {
        clearInterval(pollInterval);
      }
    };
  }, [pollInterval]);

  const startPolling = () => {
    let attempts = 0;
    const maxAttempts = 60; // 2 minutes maximum polling time
    
    const interval = setInterval(() => {
      window.FB.getLoginStatus((response) => {
        console.log('Polling status:', response.status);
        attempts++;
        
        if (response.status === 'connected') {
          clearInterval(interval);
          setIsVerifying(false);
          navigate('/success');
        } else if (attempts >= maxAttempts) {
          clearInterval(interval);
          setIsVerifying(false);
        }
      }, true); // Force fresh check
    }, 2000);

    setPollInterval(interval);
  };

  const handleFacebookLogin = () => {
    window.FB.login(function(response) {
      console.log('Login response:', response);
      
      if (response.status === 'connected') {
        // User is logged in and authorized the app
        console.log('Successfully logged in:', response);
        navigate('/success');
      } else if (response.status === 'not_authorized') {
        // User is logged into Facebook but hasn't authorized your app
        console.log('Awaiting device verification...');
        setIsVerifying(true);
        startPolling();
      } else {
        // User cancelled login or did not fully authorize
        console.log('User cancelled login or did not fully authorize.');
        setIsVerifying(false);
      }
    }, {
      scope: [
        // Facebook Pages
        'pages_show_list',
        'pages_manage_metadata', // Required dependency for pages_messaging
        'pages_messaging',
        // Instagram
        'instagram_basic',
        'instagram_manage_messages',
      ].join(','),
      
      auth_type: 'rerequest' // This forces Facebook to show the permissions dialog
    });
  };

  return (
    <div className="login-container">
      <div className="login-box">
        <h1>Conecta tus cuentas</h1>
        {isVerifying ? (
          <>
            <div className="verification-message">
              <h2>Verificación pendiente</h2>
              <p>Por favor, aprueba el inicio de sesión en tu aplicación de Facebook.</p>
              <div className="loading-spinner">
                <i className="fas fa-spinner fa-spin"></i>
              </div>
            </div>
          </>
        ) : (
          <>
            <p>Para comenzar, conecta tus cuentas de redes sociales a nuestra app</p>
            <button onClick={handleFacebookLogin} className="facebook-login-btn">
              <i className="fab fa-facebook"></i> Continuar con Facebook
            </button>
          </>
        )}
      </div>
    </div>
  );
}

export default Login;