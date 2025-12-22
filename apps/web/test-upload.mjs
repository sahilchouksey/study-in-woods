#!/usr/bin/env node
/**
 * Test script to simulate frontend batch upload with MULTIPLE PDFs
 * Run with: node test-upload.mjs
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const API_URL = 'https://api.studyinwoods.app';

async function login() {
  console.log('🔐 Logging in...');
  const response = await fetch(`${API_URL}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: 'admin@studyinwoods.app',
      password: '@Sahil2003@'
    })
  });
  
  const data = await response.json();
  if (!data.success) {
    throw new Error(`Login failed: ${data.message}`);
  }
  
  console.log('✅ Login successful');
  return data.data.access_token;
}

async function testBatchUpload(token) {
  console.log('\n📤 Testing batch upload with MULTIPLE PDFs...');
  
  // Define multiple PDF files to upload (simulating what user would select)
  const pdfFiles = [
    {
      path: path.join(__dirname, '../api/tests/testdata/MCA-302-AI-MAY-2024.pdf'),
      name: 'MCA-302-AI-MAY-2024.pdf',
      type: 'pyq'
    },
    {
      path: path.join(__dirname, '../../project-docs/01-Introduction.pdf'),
      name: '01-Introduction.pdf',
      type: 'notes'
    },
    {
      path: path.join(__dirname, '../../project-docs/02-Project-Understanding.pdf'),
      name: '02-Project-Understanding.pdf',
      type: 'notes'
    }
  ];
  
  // Create FormData exactly like the frontend does
  const formData = new FormData();
  const files = [];
  
  console.log('\n📄 Preparing files:');
  
  for (const pdfInfo of pdfFiles) {
    // Read PDF file
    const pdfBuffer = fs.readFileSync(pdfInfo.path);
    const pdfBlob = new Blob([pdfBuffer], { type: 'application/pdf' });
    
    // Simulate File object (in browser, this comes from input[type=file])
    const file = new File([pdfBlob], pdfInfo.name, { 
      type: 'application/pdf',
      lastModified: Date.now()
    });
    
    console.log(`  📄 ${pdfInfo.name}: ${file.size} bytes`);
    
    // Validate file is readable (like the frontend now does)
    try {
      const slice = file.slice(0, Math.min(100, file.size));
      const buffer = await slice.arrayBuffer();
      const firstBytes = Array.from(new Uint8Array(buffer.slice(0, 4)))
        .map(b => b.toString(16).padStart(2, '0')).join(' ');
      console.log(`     ✅ Readable, first bytes: ${firstBytes}`);
    } catch (e) {
      console.error(`     ❌ File not readable:`, e);
      return;
    }
    
    files.push({ file, type: pdfInfo.type });
  }
  
  // Append files to FormData (exactly like frontend does)
  console.log('\n📦 Appending to FormData:');
  files.forEach(({ file, type }, index) => {
    console.log(`  [${index}] Appending file: ${file.name} (${file.size} bytes)`);
    formData.append('files', file, file.name);
  });
  
  // Append types
  files.forEach(({ type }, index) => {
    console.log(`  [${index}] Appending type: ${type}`);
    formData.append('types', type);
  });
  
  // Debug FormData contents
  console.log('\n📦 FormData entries:');
  let fileCount = 0;
  let typeCount = 0;
  for (const [key, value] of formData.entries()) {
    if (value instanceof File) {
      fileCount++;
      console.log(`  files[${fileCount-1}]: File(${value.name}, ${value.size} bytes, ${value.type})`);
    } else {
      typeCount++;
      console.log(`  types[${typeCount-1}]: ${value}`);
    }
  }
  console.log(`  Total: ${fileCount} files, ${typeCount} types`);
  
  const subjectId = '24'; // Use a known subject ID
  const url = `${API_URL}/api/v1/subjects/${subjectId}/documents/batch-upload`;
  
  console.log(`\n🌐 Sending POST request to: ${url}`);
  console.log('   (This simulates exactly what the browser does)\n');
  
  const startTime = Date.now();
  
  // Use fetch exactly like the updated frontend code
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      // Don't set Content-Type - let browser/node set it with boundary
    },
    body: formData,
  });
  
  const elapsed = Date.now() - startTime;
  console.log(`📥 Response received in ${elapsed}ms`);
  console.log(`📥 Response status: ${response.status} ${response.statusText}`);
  console.log(`📥 Response headers:`);
  for (const [key, value] of response.headers.entries()) {
    if (['content-type', 'x-request-id', 'x-ratelimit-remaining'].includes(key)) {
      console.log(`     ${key}: ${value}`);
    }
  }
  
  if (!response.ok) {
    const errorText = await response.text();
    console.error('\n❌ Upload failed:', response.status, errorText);
    return;
  }
  
  const data = await response.json();
  console.log('\n✅ Upload response:');
  console.log(JSON.stringify(data, null, 2));
  
  if (data.success && data.data) {
    console.log('\n📊 Summary:');
    console.log(`   Job ID: ${data.data.job_id}`);
    console.log(`   Status: ${data.data.status}`);
    console.log(`   Total Items: ${data.data.total_items}`);
    console.log(`   Message: ${data.data.message}`);
  }
  
  return data;
}

async function main() {
  try {
    const token = await login();
    await testBatchUpload(token);
  } catch (error) {
    console.error('\n❌ Error:', error);
    process.exit(1);
  }
}

main();
