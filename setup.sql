CREATE TABLE fountains (
     id MEDIUMINT NOT NULL AUTO_INCREMENT,
     location POINT NOT NULL,
     state ENUM('usable','faulty') NOT NULL,
     PRIMARY KEY (id)
);
