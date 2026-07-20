// Каталог товаров и счетов для нагрузки.
//
// Раньше всеVU били в один sku-1 и один acc-1, то есть каждый заказ конкурировал
// за одну и ту же строку склада и одну и ту же строку баланса. Это худший
// возможный случай по блокировкам, и он маскировал всё остальное: упирались в
// row lock, а выглядело как потолок системы.
//
// Здесь два режима. Zipf — реалистичный спрос: доля обращений к i-му по
// популярности SKU пропорциональна 1/i^s, верхушка каталога забирает основной
// трафик, хвост почти простаивает. Hot key — флеш-сейл: заданная доля запросов
// принудительно идёт в один SKU.

export function loadCatalog() {
  return {
    skuCount: Number(__ENV.SKU_COUNT || 200),
    accountCount: Number(__ENV.ACCOUNT_COUNT || 200),
    zipfS: Number(__ENV.ZIPF_S || 1.0),
    hotSku: __ENV.HOT_SKU || "",
    hotShare: Number(__ENV.HOT_SHARE || 0),
  };
}

// Кумулятивные веса Zipf строятся один раз на VU. Тысяча float-ов на VU —
// это ничто по памяти, зато выборка идёт по обычному массиву без обращений
// к SharedArray на каждой итерации.
export function buildZipfCdf(count, s) {
  const cdf = new Array(count);
  let total = 0;
  for (let i = 0; i < count; i++) {
    total += 1 / Math.pow(i + 1, s);
    cdf[i] = total;
  }
  for (let i = 0; i < count; i++) {
    cdf[i] /= total;
  }
  return cdf;
}

// Обратное преобразование: бинарный поиск по кумулятивным весам.
function sampleZipf(cdf) {
  const r = Math.random();
  let lo = 0;
  let hi = cdf.length - 1;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (cdf[mid] < r) {
      lo = mid + 1;
    } else {
      hi = mid;
    }
  }
  return lo;
}

export function pickSku(catalog, cdf) {
  if (catalog.hotSku && Math.random() < catalog.hotShare) {
    return catalog.hotSku;
  }
  return `sku-${sampleZipf(cdf) + 1}`;
}

// Счета выбираются равномерно: цель — развести конкуренцию за баланс, а не
// воспроизвести распределение богатства.
export function pickAccount(catalog) {
  return `acc-${Math.floor(Math.random() * catalog.accountCount) + 1}`;
}