-- +goose Up
INSERT INTO ad_slots (id, publisher_id, name, width, height, geo, min_price)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    '22222222-2222-2222-2222-222222222222',
    'home_banner', 320, 50, 'RU', 1000000
);

INSERT INTO campaigns (id, advertiser_id, name, budget_total, budget_remaining, budget_reserved, geo_target)
VALUES
    ('33333333-3333-3333-3333-333333333333',
     '44444444-4444-4444-4444-444444444444',
     'Nike Summer', 100000000000, 100000000000, 0, 'RU'),
    ('55555555-5555-5555-5555-555555555555',
     '66666666-6666-6666-6666-666666666666',
     'Adidas Run', 100000000000, 100000000000, 0, 'RU'),
    ('77777777-7777-7777-7777-777777777777',
     '88888888-8888-8888-8888-888888888888',
     'Puma Winter', 100000000000, 100000000000, 0, 'RU');

INSERT INTO creatives (id, campaign_id, width, height, url, click_url)
VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     '33333333-3333-3333-3333-333333333333',
     320, 50,
     'https://cdn.example.com/nike.jpg',
     'https://example.com/click_nike'),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
     '55555555-5555-5555-5555-555555555555',
     320, 50,
     'https://cdn.example.com/adidas.jpg',
     'https://example.com/click_adidas'),
    ('cccccccc-cccc-cccc-cccc-cccccccccccc',
     '77777777-7777-7777-7777-777777777777',
     320, 50,
     'https://cdn.example.com/puma.jpg',
     'https://example.com/click_puma');

-- +goose Down
DELETE FROM creatives WHERE id IN (
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
    'cccccccc-cccc-cccc-cccc-cccccccccccc'
);

DELETE FROM campaigns WHERE id IN (
    '33333333-3333-3333-3333-333333333333',
    '55555555-5555-5555-5555-555555555555',
    '77777777-7777-7777-7777-777777777777'
);

DELETE FROM ad_slots WHERE id = '11111111-1111-1111-1111-111111111111';