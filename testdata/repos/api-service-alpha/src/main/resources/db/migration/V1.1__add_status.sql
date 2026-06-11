alter table thing add column status varchar(32);
alter table thing drop column created;
alter table thing_detail modify detail varchar(2048);
