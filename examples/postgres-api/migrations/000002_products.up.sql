CREATE TABLE "products" (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 "name" TEXT NOT NULL CHECK (length(btrim("name")) > 0 AND length("name") <= 255),
 "price" DOUBLE PRECISION NOT NULL,
 "active" BOOLEAN NOT NULL
);
