const dropArea = document.getElementById('drop-area');
const fileInput = document.getElementById('file-input');
const storageClassSelect = document.getElementById('storage-class');
const progressWrap = document.getElementById('progress-wrap');
const progressBar = document.getElementById('progress-bar');
const progressPct = document.getElementById('progress-pct');
const uploadError = document.getElementById('upload-error');
const resultPanel = document.getElementById('result-panel');
const previewImg = document.getElementById('preview-img');
const uploadMeta = document.getElementById('upload-meta');
const exifTable = document.getElementById('exif-table');
const exifEmpty = document.getElementById('exif-empty');
const colorSwatches = document.getElementById('color-swatches');
const mapSection = document.getElementById('map-section');
const fileList = document.getElementById('file-list');

let leafletMap = null;

// --- Upload ---

['dragenter', 'dragover'].forEach(ev =>
  dropArea.addEventListener(ev, e => { e.preventDefault(); dropArea.classList.add('hover'); }));
['dragleave', 'drop'].forEach(ev =>
  dropArea.addEventListener(ev, e => { e.preventDefault(); dropArea.classList.remove('hover'); }));
dropArea.addEventListener('drop', e => handleFiles(e.dataTransfer.files));
fileInput.addEventListener('change', e => handleFiles(e.target.files));

function handleFiles(files) {
  if (!files.length) return;
  const file = files[0];
  uploadError.hidden = true;
  previewFile(file);
  uploadFile(file);
}

function previewFile(file) {
  const reader = new FileReader();
  reader.onload = e => { previewImg.src = e.target.result; };
  reader.readAsDataURL(file);
}

function uploadFile(file) {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('storage_class', storageClassSelect.value);

  const xhr = new XMLHttpRequest();
  xhr.upload.addEventListener('progress', e => {
    if (e.lengthComputable) {
      const pct = Math.round((e.loaded / e.total) * 100);
      progressBar.style.width = pct + '%';
      progressPct.textContent = pct + '%';
    }
  });
  xhr.addEventListener('load', () => {
    progressWrap.hidden = true;
    let data;
    try { data = JSON.parse(xhr.responseText); }
    catch { showError('Unexpected server response'); return; }
    if (xhr.status >= 400 || data.error) { showError(data.error || `Upload failed (${xhr.status})`); return; }
    renderResult(file, data);
    loadFileList();
  });
  xhr.addEventListener('error', () => {
    progressWrap.hidden = true;
    showError('Network error during upload');
  });

  xhr.open('POST', '/api/upload');
  progressBar.style.width = '0%';
  progressPct.textContent = '0%';
  progressWrap.hidden = false;
  xhr.send(formData);
}

function showError(msg) {
  uploadError.textContent = msg;
  uploadError.hidden = false;
}

// --- Result rendering ---

function renderResult(file, data) {
  resultPanel.hidden = false;

  uploadMeta.textContent =
    `${file.name} · ${formatBytes(data.size_bytes)} · ${data.storage_class === 'DEEP_ARCHIVE' ? 'Glacier Deep Archive' : 'S3 Standard'} · ${data.key}`;

  // EXIF table
  const exif = data.exif || {};
  const fields = [
    ['Camera', exif.camera_model],
    ['Aperture', exif.aperture],
    ['Shutter Speed', exif.shutter_speed],
    ['ISO', exif.iso],
    ['Focal Length', exif.focal_length],
    ['Date Taken', exif.date_taken ? new Date(exif.date_taken).toLocaleString() : null],
  ].filter(([, v]) => v);

  exifTable.innerHTML = fields
    .map(([k, v]) => `<tr><th>${k}</th><td>${escapeHtml(String(v))}</td></tr>`)
    .join('');
  exifEmpty.hidden = fields.length > 0;

  // Color swatches
  colorSwatches.innerHTML = (data.colors || [])
    .map(c => `<div class="swatch" style="background:${c.hex}" title="${c.hex}"><span>${c.hex}</span></div>`)
    .join('');

  renderMap(exif);
  resultPanel.scrollIntoView({ behavior: 'smooth' });
}

function renderMap(exif) {
  if (exif.gps_lat == null || exif.gps_lng == null) {
    mapSection.hidden = true;
    return;
  }
  mapSection.hidden = false;
  if (leafletMap) {
    leafletMap.remove();
    leafletMap = null;
  }
  leafletMap = L.map('map').setView([exif.gps_lat, exif.gps_lng], 13);
  L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; OpenStreetMap contributors'
  }).addTo(leafletMap);
  L.marker([exif.gps_lat, exif.gps_lng]).addTo(leafletMap)
    .bindPopup(`${exif.gps_lat.toFixed(5)}, ${exif.gps_lng.toFixed(5)}`);
  // Leaflet needs a size recalculation when the container was just unhidden.
  setTimeout(() => leafletMap.invalidateSize(), 100);
}

// --- File list ---

async function loadFileList() {
  try {
    const res = await fetch('/api/files');
    const files = await res.json();
    if (!res.ok) throw new Error(files.error || res.statusText);
    if (!files.length) {
      fileList.innerHTML = '<li class="muted">No files uploaded yet.</li>';
      return;
    }
    fileList.innerHTML = files.map(f => {
      const name = escapeHtml(f.key);
      const link = f.retrievable && f.url
        ? `<a href="${f.url}" target="_blank" rel="noopener">${name}</a>`
        : name;
      const badge = f.storage_class === 'DEEP_ARCHIVE' || f.storage_class === 'GLACIER'
        ? '<span class="badge archive">🧊 Archived</span>'
        : '<span class="badge standard">Standard</span>';
      return `<li>
        <span class="file-key">${link}</span>
        ${badge}
        <span class="file-size">${formatBytes(f.size)}</span>
        <span class="file-date">${new Date(f.last_modified).toLocaleDateString()}</span>
      </li>`;
    }).join('');
  } catch (err) {
    fileList.innerHTML = `<li class="error">Failed to load files: ${escapeHtml(err.message)}</li>`;
  }
}

function formatBytes(n) {
  if (n == null) return '';
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
  return `${n.toFixed(i ? 1 : 0)} ${units[i]}`;
}

function escapeHtml(s) {
  return s.replace(/[&<>"']/g, c => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  }[c]));
}

loadFileList();
