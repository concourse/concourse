port module Dashboard exposing (Model, Msg, init, update, view)

import Concourse
import Concourse.Pipeline
import Dict
import Html exposing (Html)
import Html.Attributes exposing (class, href)
import RemoteData


type alias Model =
    { pipelines : RemoteData.WebData (List Concourse.Pipeline)
    }


type Msg
    = PipelinesResponse (RemoteData.WebData (List Concourse.Pipeline))


init : ( Model, Cmd Msg )
init =
    ( { pipelines = RemoteData.NotAsked }
    , fetchPipelines
    )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        PipelinesResponse response ->
            ( { model | pipelines = response }, Cmd.none )


view : Model -> Html msg
view model =
    case model.pipelines of
        RemoteData.Success pipelines ->
            let
                pipelinesByTeam =
                    List.foldl
                        (\pipeline byTeam ->
                            Dict.update pipeline.teamName
                                (\mps ->
                                    Just (pipeline :: Maybe.withDefault [] mps)
                                )
                                byTeam
                        )
                        Dict.empty
                        pipelines
            in
                Html.div [ class "dashboard" ]
                    (Dict.values (Dict.map viewGroup pipelinesByTeam))

        _ ->
            Html.text ""


viewGroup : String -> List Concourse.Pipeline -> Html msg
viewGroup teamName pipelines =
    Html.div [ class "dashboard-team-group" ]
        [ Html.div [ class "dashboard-team-name" ]
            [ Html.text teamName
            ]
        , Html.div [ class "dashboard-team-pipelines" ]
            (List.map viewPipeline pipelines)
        ]


viewPipeline : Concourse.Pipeline -> Html msg
viewPipeline pipeline =
    Html.div [ class "dashboard-pipeline" ]
        [ Html.div [ class "dashboard-pipeline-icon paused" ]
            []
        , Html.div [ class "dashboard-pipeline-name" ]
            [ Html.a [ href pipeline.url ] [ Html.text pipeline.name ] ]
        ]


fetchPipelines : Cmd Msg
fetchPipelines =
    Cmd.map PipelinesResponse <|
        RemoteData.asCmd Concourse.Pipeline.fetchPipelines
