// Trial Booking demo UI — vanilla JS, no build step.
// Consumes the versioned JSON API under /api/v1 and surfaces error envelopes.

const API = '/api/v1';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// request() calls the API and unwraps the {data} / {error} envelope.
// On an HTTP error status or an error envelope it throws an Error whose
// message is "CODE: message" so callers can render it directly.
async function request(path, options = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });

  let body = null;
  try {
    body = await res.json();
  } catch (_) {
    /* empty or non-JSON body */
  }

  if (!res.ok) {
    const code = body && body.error ? body.error.code : 'http_' + res.status;
    const message = body && body.error ? body.error.message : 'request failed';
    throw new Error(`${code}: ${message}`);
  }

  return body ? body.data : null;
}

function el(id) {
  return document.getElementById(id);
}

function setDisabled(node, disabled) {
  node.disabled = disabled;
}

// Render an error into a target element with a consistent style.
function renderError(target, err) {
  target.className = 'mt-2 text-sm text-red-600 font-medium';
  target.textContent = `Error: ${err.message}`;
}

// ---------------------------------------------------------------------------
// Parent Booking
// ---------------------------------------------------------------------------

let currentBookingId = null;

async function loadClasses() {
  const select = el('class-select');
  const rosterSelect = el('roster-class-select');

  const classes = await request(`${API}/classes`);

  const render = (node, placeholder) => {
    const previous = node.value;
    node.innerHTML = '';
    const opt = document.createElement('option');
    opt.value = '';
    opt.textContent = placeholder;
    opt.disabled = true;
    opt.selected = true;
    node.appendChild(opt);

    for (const c of classes) {
      const o = document.createElement('option');
      o.value = c.id;
      o.textContent = `${c.title} — ${c.remaining_seats}/${c.capacity} seats left`;
      node.appendChild(o);
    }

    // Preserve the user's selection across refreshes (e.g. after a payment).
    if (previous) node.value = previous;
  };

  render(select, 'Select a class…');
  render(rosterSelect, 'Select a class…');

  // The class dropdowns start disabled in the markup; enable them once the
  // options are actually available.
  setDisabled(select, false);
  setDisabled(rosterSelect, false);
}

async function loadChildren(parentId) {
  const select = el('child-select');
  setDisabled(select, true);
  select.innerHTML = '';

  const parent = await request(`${API}/parents/${parentId}`);

  const opt = document.createElement('option');
  opt.value = '';
  opt.textContent = 'Select a child…';
  opt.disabled = true;
  opt.selected = true;
  select.appendChild(opt);

  for (const child of parent.children) {
    const o = document.createElement('option');
    o.value = child.id;
    o.textContent = child.name;
    select.appendChild(o);
  }

  setDisabled(select, false);
}

async function createBooking() {
  const result = el('booking-result');
  const bookBtn = el('book-btn');
  const classSelect = el('class-select');
  const childSelect = el('child-select');

  const parentId = Number(el('parent-select').value);
  const studentId = Number(childSelect.value);
  const trialClassId = Number(classSelect.value);

  if (!studentId || !trialClassId) {
    result.className = 'text-sm text-red-600';
    result.textContent = 'Please select a child and a class.';
    return;
  }

  setDisabled(bookBtn, true);
  result.className = 'text-sm text-slate-500';
  result.textContent = 'Booking…';

  try {
    const booking = await request(`${API}/bookings`, {
      method: 'POST',
      body: JSON.stringify({
        parent_id: parentId,
        student_id: studentId,
        trial_class_id: trialClassId,
      }),
    });

    currentBookingId = booking.id;
    result.className = 'text-sm text-emerald-600 font-medium';
    result.innerHTML =
      `Booking created! ID: <span class="font-bold">${booking.id}</span>. ` +
      `Status: <span class="font-bold">${booking.status}</span>.`;

    el('payment-controls').classList.remove('hidden');
    el('payment-result').textContent = '';
  } catch (err) {
    renderError(result, err);
    hidePaymentControls();
  } finally {
    setDisabled(bookBtn, false);
  }
}

async function processPayment(outcome) {
  if (!currentBookingId) return;

  const result = el('payment-result');
  const successBtn = el('pay-success-btn');
  const failureBtn = el('pay-failure-btn');

  setDisabled(successBtn, true);
  setDisabled(failureBtn, true);
  result.textContent = 'Processing payment…';

  try {
    const booking = await request(`${API}/bookings/${currentBookingId}/payment`, {
      method: 'POST',
      body: JSON.stringify({
        outcome,
        provider_ref: `ui_demo_${Date.now()}`,
      }),
    });

    if (booking.status === 'confirmed') {
      result.className = 'text-sm text-emerald-600 font-medium';
      result.textContent = 'Booking Confirmed!';
    } else {
      result.className = 'text-sm text-amber-600 font-medium';
      result.textContent = `Status: ${booking.status}`;
    }
    hidePaymentControls();
  } catch (err) {
    renderError(result, err);
    hidePaymentControls();
  }

  // Refresh class availability after a payment settles a seat.
  loadClasses().catch(() => {});
}

function hidePaymentControls() {
  el('payment-controls').classList.add('hidden');
}

// ---------------------------------------------------------------------------
// Admin Roster
// ---------------------------------------------------------------------------

// statusBadge returns a Tailwind-colored badge for a booking status.
function statusBadge(status) {
  const styles = {
    confirmed: 'bg-emerald-100 text-emerald-800',
    pending_payment: 'bg-amber-100 text-amber-800',
    payment_failed: 'bg-red-100 text-red-800',
    cancelled: 'bg-slate-200 text-slate-700',
  };
  const cls = styles[status] || 'bg-slate-200 text-slate-700';
  return `<span class="inline-block px-2 py-0.5 rounded-full text-xs font-medium ${cls}">${escapeHtml(status)}</span>`;
}

async function loadRoster() {
  const classId = Number(el('roster-class-select').value);
  const summary = el('roster-summary');
  const table = el('roster-table');

  if (!classId) {
    table.textContent = 'Select a class and load the roster.';
    summary.textContent = '';
    return;
  }

  el('roster-btn').disabled = true;
  table.textContent = 'Loading roster…';

  try {
    const roster = await request(`${API}/admin/classes/${classId}/roster`);

    summary.innerHTML =
      `Class "<span class="font-medium">${escapeHtml(roster.class.title)}</span>" — ` +
      `Confirmed: <span class="font-medium">${roster.confirmed_count}/${roster.class.capacity}</span>, ` +
      `Remaining seats: <span class="font-medium">${roster.remaining_seats}</span>. ` +
      `<span class="text-slate-400">Only confirmed bookings count toward capacity.</span>`;

    if (!roster.entries.length) {
      table.textContent = 'No bookings for this class yet.';
      return;
    }

    const rows = roster.entries
      .map(
        (e) => `
          <tr class="border-b border-slate-100">
            <td class="py-2 pr-4">${statusBadge(e.status)}</td>
            <td class="py-2 pr-4">${escapeHtml(e.student_name)}</td>
            <td class="py-2 pr-4">${escapeHtml(e.parent_name)}</td>
            <td class="py-2 pr-4">${escapeHtml(e.parent_email)}</td>
            <td class="py-2">${formatDate(e.created_at)}</td>
          </tr>`
      )
      .join('');

    table.innerHTML = `
      <table class="w-full text-sm">
        <thead>
          <tr class="text-left text-slate-500 border-b border-slate-200">
            <th class="py-2 pr-4">Status</th>
            <th class="py-2 pr-4">Student</th>
            <th class="py-2 pr-4">Parent</th>
            <th class="py-2 pr-4">Parent Email</th>
            <th class="py-2">Booked At</th>
          </tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>`;
  } catch (err) {
    renderError(table, err);
  } finally {
    el('roster-btn').disabled = false;
  }
}

function escapeHtml(str) {
  return String(str)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function formatDate(iso) {
  const d = new Date(iso);
  return isNaN(d.getTime()) ? iso : d.toLocaleString();
}

// ---------------------------------------------------------------------------
// Tabs & bootstrap
// ---------------------------------------------------------------------------

function showTab(tab) {
  const isParent = tab === 'parent';
  el('panel-parent').classList.toggle('hidden', !isParent);
  el('panel-admin').classList.toggle('hidden', isParent);

  el('tab-parent').className = isParent
    ? 'px-4 py-2 rounded-md font-medium bg-slate-900 text-white'
    : 'px-4 py-2 rounded-md font-medium bg-white border border-slate-300 text-slate-700';
  el('tab-admin').className = isParent
    ? 'px-4 py-2 rounded-md font-medium bg-white border border-slate-300 text-slate-700'
    : 'px-4 py-2 rounded-md font-medium bg-slate-900 text-white';
}

async function init() {
  // Parent dropdown: hardcoded demo parents.
  const parentSelect = el('parent-select');
  parentSelect.innerHTML = '';
  for (const [id, name] of [[1, 'Sarah Mitchell'], [2, 'James Okafor']]) {
    const o = document.createElement('option');
    o.value = id;
    o.textContent = name;
    parentSelect.appendChild(o);
  }

  el('tab-parent').addEventListener('click', () => showTab('parent'));
  el('tab-admin').addEventListener('click', () => showTab('admin'));

  el('parent-select').addEventListener('change', async (e) => {
    try {
      await loadChildren(Number(e.target.value));
    } catch (err) {
      renderError(el('booking-result'), err);
    }
  });

  el('book-btn').addEventListener('click', createBooking);
  el('pay-success-btn').addEventListener('click', () => processPayment('success'));
  el('pay-failure-btn').addEventListener('click', () => processPayment('failure'));
  el('roster-btn').addEventListener('click', loadRoster);

  try {
    await loadClasses();
    await loadChildren(1);
    el('book-btn').disabled = false;
  } catch (err) {
    renderError(el('booking-result'), err);
  }
}

document.addEventListener('DOMContentLoaded', init);
