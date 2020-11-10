module RouteBuilder exposing (RouteBuilder, append, appendPath, appendQuery, build, pipeline)

import Concourse
import DotNotation
import Url.Builder


type alias RouteBuilder =
    ( List String, List Url.Builder.QueryParameter )


append : RouteBuilder -> RouteBuilder -> RouteBuilder
append ( p1, q1 ) ( p2, q2 ) =
    ( p2 ++ p1, q2 ++ q1 )


appendPath : List String -> RouteBuilder -> RouteBuilder
appendPath path base =
    append ( path, [] ) base


appendQuery : List Url.Builder.QueryParameter -> RouteBuilder -> RouteBuilder
appendQuery query base =
    append ( [], query ) base


build : RouteBuilder -> String
build ( path, query ) =
    Url.Builder.absolute path query


pipeline : { r | teamName : String, pipelineName : String, pipelineInstanceVars : Concourse.InstanceVars } -> RouteBuilder
pipeline id =
    ( [ "teams", id.teamName, "pipelines", id.pipelineName ]
    , DotNotation.flatten id.pipelineInstanceVars
        |> List.map
            (\var ->
                let
                    ( k, v ) =
                        DotNotation.serialize var
                in
                Url.Builder.string ("vars." ++ k) v
            )
    )
