// SRDB WebUI - htmx 版本

// 全局状态
window.srdbState = {
  selectedTable: null,
  currentPage: 1,
  pageSize: 20,
  selectedColumns: [],
  expandedTables: new Set(),
  expandedLevels: new Set([0, 1]),
};

// 选择表格
function selectTable(tableName) {
  window.srdbState.selectedTable = tableName;
  window.srdbState.currentPage = 1;

  // 高亮选中的表
  document.querySelectorAll(".table-item").forEach((el) => {
    el.classList.toggle("selected", el.dataset.table === tableName);
  });

  // 加载表数据
  loadTableData(tableName);
}

// 加载表数据
function loadTableData(tableName) {
  const mainContent = document.getElementById("main-content");
  mainContent.innerHTML = '<div class="loading">Loading...</div>';

  fetch(
    `/api/tables-view/${tableName}?page=${window.srdbState.currentPage}&pageSize=${window.srdbState.pageSize}`,
  )
    .then((res) => res.text())
    .then((html) => {
      mainContent.innerHTML = html;
    })
    .catch((err) => {
      console.error("Failed to load table data:", err);
      mainContent.innerHTML =
        '<div class="error">Failed to load table data</div>';
    });
}

// 切换视图 (Data / Manifest)
function switchView(tableName, mode) {
  const mainContent = document.getElementById("main-content");
  mainContent.innerHTML = '<div class="loading">Loading...</div>';

  const endpoint =
    mode === "manifest"
      ? `/api/tables-view/${tableName}/manifest`
      : `/api/tables-view/${tableName}?page=${window.srdbState.currentPage}&pageSize=${window.srdbState.pageSize}`;

  fetch(endpoint)
    .then((res) => res.text())
    .then((html) => {
      mainContent.innerHTML = html;
    });
}

// 分页
function changePage(delta) {
  window.srdbState.currentPage += delta;
  if (window.srdbState.selectedTable) {
    loadTableData(window.srdbState.selectedTable);
  }
}

function jumpToPage(page) {
  window.srdbState.currentPage = parseInt(page);
  if (window.srdbState.selectedTable) {
    loadTableData(window.srdbState.selectedTable);
  }
}

function changePageSize(newSize) {
  window.srdbState.pageSize = parseInt(newSize);
  window.srdbState.currentPage = 1;
  if (window.srdbState.selectedTable) {
    loadTableData(window.srdbState.selectedTable);
  }
}

// Modal 相关
function showModal(title, content) {
  document.getElementById("modal-title").textContent = title;
  document.getElementById("modal-body-content").textContent = content;
  document.getElementById("modal").style.display = "flex";
}

function closeModal() {
  document.getElementById("modal").style.display = "none";
}

function showCellContent(content) {
  showModal("Cell Content", content);
}

function showRowDetail(tableName, seq) {
  fetch(`/api/tables/${tableName}/data/${seq}`)
    .then((res) => res.json())
    .then((data) => {
      const formatted = JSON.stringify(data, null, 2);
      showModal(`Row Detail (Seq: ${seq})`, formatted);
    })
    .catch((err) => {
      console.error("Failed to load row detail:", err);
      alert("Failed to load row detail");
    });
}

// 折叠展开
function toggleExpand(tableName) {
  const item = document.querySelector(`[data-table="${tableName}"]`);
  const fieldsDiv = item.querySelector(".schema-fields");
  const icon = item.querySelector(".expand-icon");

  if (window.srdbState.expandedTables.has(tableName)) {
    window.srdbState.expandedTables.delete(tableName);
    fieldsDiv.style.display = "none";
    icon.classList.remove("expanded");
  } else {
    window.srdbState.expandedTables.add(tableName);
    fieldsDiv.style.display = "block";
    icon.classList.add("expanded");
  }
}

function toggleLevel(level) {
  const levelCard = document.querySelector(`[data-level="${level}"]`);
  const fileList = levelCard.querySelector(".file-list");
  const icon = levelCard.querySelector(".expand-icon");

  if (window.srdbState.expandedLevels.has(level)) {
    window.srdbState.expandedLevels.delete(level);
    fileList.style.display = "none";
    icon.classList.remove("expanded");
  } else {
    window.srdbState.expandedLevels.add(level);
    fileList.style.display = "grid";
    icon.classList.add("expanded");
  }
}

// 格式化工具
function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(2) + " " + sizes[i];
}

function formatCount(count) {
  if (count >= 1000000) return (count / 1000000).toFixed(1) + "M";
  if (count >= 1000) return (count / 1000).toFixed(1) + "K";
  return count.toString();
}

// 点击 modal 外部关闭
document.addEventListener("click", (e) => {
  const modal = document.getElementById("modal");
  if (e.target === modal) {
    closeModal();
  }
});

// ESC 键关闭 modal
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    closeModal();
  }
});

// 切换列显示
function toggleColumn(columnName) {
  // 切换 schema-field-card 的选中状态
  const card = document.querySelector(
    `.schema-field-card[data-column="${columnName}"]`,
  );
  if (!card) return;

  card.classList.toggle("selected");
  const isSelected = card.classList.contains("selected");

  // 切换表格列的显示/隐藏
  const headers = document.querySelectorAll(`th[data-column="${columnName}"]`);
  const cells = document.querySelectorAll(`td[data-column="${columnName}"]`);

  headers.forEach((header) => {
    header.style.display = isSelected ? "" : "none";
  });

  cells.forEach((cell) => {
    cell.style.display = isSelected ? "" : "none";
  });
}
