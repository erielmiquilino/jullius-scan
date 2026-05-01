import { test, expect } from "../fixtures";
import type { ItemSearchResponse } from "../../src/support/types";

const HOUSE_ID = 1;

const daysAgo = (days: number): Date => {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() - days);
  return d;
};

const search = async (
  api: import("@playwright/test").APIRequestContext,
  bearerToken: string,
  params: { q: string; periodDays?: number | null },
) => {
  const search = new URLSearchParams();
  search.set("q", params.q);
  if (params.periodDays === null) {
    search.set("period_days", "null");
  } else if (typeof params.periodDays === "number") {
    search.set("period_days", String(params.periodDays));
  }
  return api.get(`/api/v1/items/search?${search.toString()}`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
};

test.describe("GET /api/v1/items/search", () => {
  test("agrega itens com mesmo barcode em um único resultado", async ({ api, bearerToken, state }) => {
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "11111111000111", name: "Mercado A" },
      [{ description: "BANANA NANICA KG", barcode: "7891234567890", quantity: 1, unit: "KG", unitPrice: 4.5 }],
      { issuedAt: daysAgo(10) },
    );
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "11111111000111", name: "Mercado A" },
      [{ description: "BANANA NANICA", barcode: "7891234567890", quantity: 2, unit: "KG", unitPrice: 5.0 }],
      { issuedAt: daysAgo(2) },
    );

    const response = await search(api, bearerToken, { q: "banana" });
    expect(response.status()).toBe(200);
    const body = (await response.json()) as ItemSearchResponse;

    expect(body.items).toHaveLength(1);
    expect(body.items[0].barcode).toBe("7891234567890");
    expect(body.items[0].purchase_count).toBe(2);
    expect(body.items[0].last_unit_price).toBeCloseTo(5.0);
    expect(body.items[0].previous_unit_price).toBeCloseTo(4.5);
    expect(body.query.matched_by).toBe("description");
  });

  test("mantém itens sem barcode separados por descrição literal", async ({ api, bearerToken, state }) => {
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "22222222000122", name: "Mercado B" },
      [{ description: "DETERGENTE YPE 500ML", barcode: null, unitPrice: 3.0 }],
      { issuedAt: daysAgo(5) },
    );
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "22222222000122", name: "Mercado B" },
      [{ description: "DETERGENTE YPE", barcode: null, unitPrice: 2.8 }],
      { issuedAt: daysAgo(3) },
    );

    const response = await search(api, bearerToken, { q: "detergente" });
    expect(response.status()).toBe(200);
    const body = (await response.json()) as ItemSearchResponse;

    expect(body.items).toHaveLength(2);
    const descriptions = body.items.map((it) => it.description).sort();
    expect(descriptions).toEqual(["DETERGENTE YPE", "DETERGENTE YPE 500ML"]);
  });

  test("filtra por período padrão de 30 dias (60d ignorado, 5d incluído)", async ({ api, bearerToken, state }) => {
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "33333333000133", name: "Mercado C" },
      [{ description: "ARROZ TIPO 1", barcode: null, unitPrice: 25.0 }],
      { issuedAt: daysAgo(60) },
    );
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "33333333000133", name: "Mercado C" },
      [{ description: "ARROZ TIPO 1", barcode: null, unitPrice: 27.0 }],
      { issuedAt: daysAgo(5) },
    );

    const response = await search(api, bearerToken, { q: "arroz" });
    expect(response.status()).toBe(200);
    const body = (await response.json()) as ItemSearchResponse;

    expect(body.items).toHaveLength(1);
    expect(body.items[0].purchase_count).toBe(1);
    expect(body.items[0].last_unit_price).toBeCloseTo(27.0);
    expect(body.items[0].previous_unit_price).toBeUndefined();
  });

  test("período ampliado (period_days=90) inclui compra de 60 dias atrás", async ({ api, bearerToken, state }) => {
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "33333333000133", name: "Mercado C" },
      [{ description: "ARROZ TIPO 1", barcode: null, unitPrice: 25.0 }],
      { issuedAt: daysAgo(60) },
    );
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "33333333000133", name: "Mercado C" },
      [{ description: "ARROZ TIPO 1", barcode: null, unitPrice: 27.0 }],
      { issuedAt: daysAgo(5) },
    );

    const response = await search(api, bearerToken, { q: "arroz", periodDays: 90 });
    expect(response.status()).toBe(200);
    const body = (await response.json()) as ItemSearchResponse;

    expect(body.items).toHaveLength(1);
    expect(body.items[0].purchase_count).toBe(2);
    expect(body.items[0].previous_unit_price).toBeCloseTo(25.0);
  });

  test("busca numérica com 8+ dígitos casa exato em barcode", async ({ api, bearerToken, state }) => {
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "44444444000144", name: "Mercado D" },
      [
        { description: "REFRIGERANTE COLA 2L", barcode: "7891000100103", unitPrice: 9.99 },
        { description: "REFRIGERANTE GUARANA 2L", barcode: "7891000100110", unitPrice: 8.99 },
      ],
      { issuedAt: daysAgo(2) },
    );

    const response = await search(api, bearerToken, { q: "7891000100103" });
    expect(response.status()).toBe(200);
    const body = (await response.json()) as ItemSearchResponse;

    expect(body.query.matched_by).toBe("barcode");
    expect(body.items).toHaveLength(1);
    expect(body.items[0].barcode).toBe("7891000100103");
    expect(body.items[0].description).toBe("REFRIGERANTE COLA 2L");
  });

  test("isolation: item de outra casa não aparece nos resultados", async ({ api, bearerToken, state }) => {
    const otherHouseId = await state.postgres.seedHouse("Outra Casa");
    await state.postgres.seedReceiptWithItems(
      otherHouseId,
      { cnpj: "55555555000155", name: "Mercado da Outra Casa" },
      [{ description: "BANANA PRATA", barcode: null, unitPrice: 6.0 }],
      { issuedAt: daysAgo(1) },
    );

    const response = await search(api, bearerToken, { q: "banana" });
    expect(response.status()).toBe(200);
    const body = (await response.json()) as ItemSearchResponse;

    expect(body.items).toHaveLength(0);
  });

  test("query com menos de 3 caracteres retorna 400", async ({ api, bearerToken }) => {
    const response = await search(api, bearerToken, { q: "ab" });
    expect(response.status()).toBe(400);
  });

  test("period_days negativo retorna 400", async ({ api, bearerToken }) => {
    const response = await search(api, bearerToken, { q: "banana", periodDays: -1 });
    expect(response.status()).toBe(400);
  });

  test("requisição não autenticada retorna 401", async ({ api }) => {
    const response = await api.get("/api/v1/items/search?q=banana");
    expect(response.status()).toBe(401);
  });

  test("previous_unit_price é null quando há apenas uma compra no período", async ({ api, bearerToken, state }) => {
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "66666666000166", name: "Mercado E" },
      [{ description: "PAO FRANCES", barcode: null, unitPrice: 0.75, quantity: 6, totalPrice: 4.5 }],
      { issuedAt: daysAgo(1) },
    );

    const response = await search(api, bearerToken, { q: "pao" });
    expect(response.status()).toBe(200);
    const body = (await response.json()) as ItemSearchResponse;

    expect(body.items).toHaveLength(1);
    expect(body.items[0].purchase_count).toBe(1);
    expect(body.items[0].previous_unit_price).toBeUndefined();
  });

  test("average_unit_price é ponderado por quantidade (2kg@4 + 1kg@7 → 5.00)", async ({ api, bearerToken, state }) => {
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "77777777000177", name: "Mercado F" },
      [{ description: "TOMATE", barcode: null, quantity: 2, unit: "KG", unitPrice: 4.0, totalPrice: 8.0 }],
      { issuedAt: daysAgo(7) },
    );
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "77777777000177", name: "Mercado F" },
      [{ description: "TOMATE", barcode: null, quantity: 1, unit: "KG", unitPrice: 7.0, totalPrice: 7.0 }],
      { issuedAt: daysAgo(2) },
    );

    const response = await search(api, bearerToken, { q: "tomate" });
    expect(response.status()).toBe(200);
    const body = (await response.json()) as ItemSearchResponse;

    expect(body.items).toHaveLength(1);
    expect(body.items[0].average_unit_price).toBeCloseTo(5.0, 2);
    expect(body.items[0].last_unit_price).toBeCloseTo(7.0);
    expect(body.items[0].previous_unit_price).toBeCloseTo(4.0);
  });

  test("busca por descrição é diacrítico-insensitive (acai casa AÇAÍ)", async ({ api, bearerToken, state }) => {
    await state.postgres.seedReceiptWithItems(
      HOUSE_ID,
      { cnpj: "88888888000188", name: "Mercado G" },
      [{ description: "AÇAÍ POLPA 500G", barcode: null, unitPrice: 12.0 }],
      { issuedAt: daysAgo(3) },
    );

    const response = await search(api, bearerToken, { q: "acai" });
    expect(response.status()).toBe(200);
    const body = (await response.json()) as ItemSearchResponse;

    expect(body.items).toHaveLength(1);
    expect(body.items[0].description).toBe("AÇAÍ POLPA 500G");
  });
});
