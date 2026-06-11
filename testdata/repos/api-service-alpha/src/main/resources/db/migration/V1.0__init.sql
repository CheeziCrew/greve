create table thing
(
    id         bigint not null auto_increment,
    name       varchar(255) not null,
    created    datetime(6),
    primary key (id)
) engine = InnoDB;

create table thing_detail
(
    id       bigint not null auto_increment,
    thing_id bigint not null,
    detail   varchar(1024),
    primary key (id)
) engine = InnoDB;

alter table if exists thing_detail
    add constraint fk_thing_detail_thing_id
        foreign key (thing_id)
            references thing (id);

create index thing_name_index on thing (name);
