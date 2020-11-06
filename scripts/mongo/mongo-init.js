db = db.getSiblingDB('test');
db.createUser(
    {
        user: 'admin',
        pwd: '123',
        roles: [{ role: 'readWrite', db: 'test' }],
    },
);