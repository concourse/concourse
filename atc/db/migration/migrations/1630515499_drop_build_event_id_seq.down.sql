do $$
declare
    b record;
    startValue int;
begin
    for b in
        select id, name, pipeline_id, team_id from builds where (status = 'started' OR status = 'pending') and completed = false
    loop
        raise notice 'dropping sequence build_event_id_seq_% ...', b.id;
        execute 'drop sequence if exists build_event_id_seq_' || b.id;

        if b.name = 'check' then
            execute 'select max(event_id) from check_build_events where build_id=' || b.id into startValue;
        elsif b.pipeline_id is null or b.pipeline_id = 0 then
            execute 'select max(event_id) from team_build_events_' || b.team_id || ' where build_id=' || b.id into startValue;
        else
            execute 'select max(event_id) from pipeline_build_events_' || b.pipeline_id || ' where build_id=' || b.id into startValue;
        end if;

        if startValue is null then
            startValue := 0;
        else
            startValue := startValue + 1;
        end if;

        raise notice 'creating sequence build_event_id_seq_% ...', b.id;
        execute 'create sequence build_event_id_seq_' || b.id || ' minvalue 0 start with ' || startValue;
    end loop;
end; $$
