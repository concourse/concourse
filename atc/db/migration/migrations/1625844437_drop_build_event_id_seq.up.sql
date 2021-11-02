do $$
declare
    b record;
begin
    for b in
        select id from builds where status = 'started'
    loop
        raise notice 'dropping sequence build_event_id_seq_% ...', b.id;
        execute 'drop sequence if exists build_event_id_seq_' || b.id;
    end loop;
end; $$
