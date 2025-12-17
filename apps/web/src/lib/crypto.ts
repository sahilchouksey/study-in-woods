import CryptoJS from 'crypto-js';

// Generate a unique salt for this browser session
const getUserSalt = (): string => {
  if (typeof window === 'undefined') {
    // Return a temporary salt for SSR - actual salt will be used on client
    return 'ssr-temp-salt';
  }
  
  let salt = localStorage.getItem('study_woods_salt');
  if (!salt) {
    salt = CryptoJS.lib.WordArray.random(128/8).toString(CryptoJS.enc.Hex);
    localStorage.setItem('study_woods_salt', salt);
  }
  return salt;
};

// Generate encryption key from browser fingerprint and salt
const getEncryptionKey = (): string => {
  if (typeof window === 'undefined') {
    // Return a placeholder for SSR
    return 'ssr-placeholder-key';
  }
  
  const salt = getUserSalt();
  const browserFingerprint = [
    navigator.userAgent,
    navigator.language,
    screen.width + 'x' + screen.height,
    new Date().getTimezoneOffset().toString(),
  ].join('|');
  
  return CryptoJS.PBKDF2(browserFingerprint, salt, {
    keySize: 256/32,
    iterations: 10000
  }).toString();
};

/**
 * Encrypts sensitive data using AES encryption
 * Data is encrypted with a key derived from browser fingerprint + salt
 */
export const encryptData = (data: string): string => {
  try {
    const key = getEncryptionKey();
    const encrypted = CryptoJS.AES.encrypt(data, key).toString();
    return encrypted;
  } catch (error) {
    console.error('Encryption failed:', error);
    throw new Error('Failed to encrypt data');
  }
};

/**
 * Decrypts data that was encrypted with encryptData
 */
export const decryptData = (encryptedData: string): string => {
  try {
    const key = getEncryptionKey();
    const decrypted = CryptoJS.AES.decrypt(encryptedData, key);
    const originalData = decrypted.toString(CryptoJS.enc.Utf8);
    
    if (!originalData) {
      throw new Error('Invalid decryption key or corrupted data');
    }
    
    return originalData;
  } catch (error) {
    console.error('Decryption failed:', error);
    throw new Error('Failed to decrypt data');
  }
};

/**
 * Securely clears encryption salt (useful for logout/reset)
 */
export const clearEncryptionSalt = (): void => {
  if (typeof window !== 'undefined') {
    localStorage.removeItem('study_woods_salt');
  }
};

/**
 * Validates if encrypted data can be properly decrypted
 */
export const validateEncryptedData = (encryptedData: string): boolean => {
  try {
    decryptData(encryptedData);
    return true;
  } catch {
    return false;
  }
};