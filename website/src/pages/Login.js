// Login.js
import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import './Login.css';

function Login() {
  const navigate = useNavigate();
  const [isVerifying, setIsVerifying] = useState(false);
  const [pollInterval, setPollInterval] = useState(null);

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
        attempts++;
        
        if (response.status === 'connected') {
          clearInterval(interval);
          setIsVerifying(false);
          const token = response.authResponse.accessToken;
          navigate('/success', { state: { accessToken: token } });
        } else if (attempts >= maxAttempts) {
          clearInterval(interval);
          setIsVerifying(false);
        }
      }, true);
    }, 2000);

    setPollInterval(interval);
  };

  const handleFacebookLogin = () => {
    window.FB.login(function(response) {
      console.log('Login response:', response);
      
      if (response.status === 'connected') {
        const token = response.authResponse.accessToken;
        console.log('Successfully logged in with token');
        navigate('/success', { state: { accessToken: token } });
      } else if (response.status === 'not_authorized') {
        console.log('Awaiting device verification...');
        setIsVerifying(true);
        startPolling();
      } else {
        console.log('User cancelled login or did not fully authorize.');
        setIsVerifying(false);
      }
    }, {
      scope: [
        'email',
        'pages_show_list',
        'pages_manage_metadata',
        'pages_messaging',
        'instagram_basic',
        'instagram_manage_messages',
      ].join(','),
      auth_type: 'rerequest'
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