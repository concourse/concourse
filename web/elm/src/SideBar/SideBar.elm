module SideBar.SideBar exposing (update, view)

import Dict exposing (Dict)
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (href)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import Routes
import SideBar.Styles as Styles


type alias Model m t p =
    { m
        | groupToggleStates : Dict String Bool
        , groups : List (Team t p)
        , hovered : Maybe DomID
    }


type alias Team t p =
    { t
        | teamName : String
        , pipelines : List (Pipeline p)
    }


type alias Pipeline p =
    { p | name : String }


update : Message -> ET (Model m g p)
update msg ( model, effects ) =
    case msg of
        Click (SideBarTeam teamName) ->
            ( { model
                | groupToggleStates =
                    model.groupToggleStates
                        |> Dict.update teamName (Maybe.map not)
              }
            , effects
            )

        _ ->
            ( model, effects )


view : Model m g p -> Html Message
view model =
    Html.div
        Styles.sideBar
        (model.groups
            |> List.filterMap
                (\g ->
                    Dict.get g.teamName model.groupToggleStates
                        |> Maybe.map (team model g)
                )
        )


team :
    { a | hovered : Maybe DomID }
    -> Team t p
    -> Bool
    -> Html Message
team { hovered } g isExpanded =
    if isExpanded then
        Html.div
            Styles.column
            [ teamHeader hovered g isExpanded
            , Html.div Styles.column <|
                List.map
                    (pipeline hovered g.teamName)
                    g.pipelines
            ]

    else
        teamHeader hovered g isExpanded


teamHeader : Maybe DomID -> Team t p -> Bool -> Html Message
teamHeader hovered t isExpanded =
    let
        isHovered =
            hovered == Just (SideBarTeam t.teamName)
    in
    Html.div
        (Styles.teamHeader
            ++ [ onClick <| Click <| SideBarTeam t.teamName
               , onMouseEnter <| Hover <| Just <| SideBarTeam t.teamName
               , onMouseLeave <| Hover Nothing
               ]
        )
        [ Html.div
            Styles.iconGroup
            [ Styles.teamIcon isHovered
            , Styles.arrow
                { isHovered = isHovered
                , isExpanded = isExpanded
                }
            ]
        , Html.div
            (Styles.teamName
                { isHovered = isHovered
                , isExpanded = isExpanded
                }
            )
            [ Html.text t.teamName ]
        ]


pipeline : Maybe DomID -> String -> Pipeline p -> Html Message
pipeline hovered teamName p =
    let
        pipelineId =
            { pipelineName = p.name
            , teamName = teamName
            }

        isHovered =
            hovered == Just (SideBarPipeline pipelineId)
    in
    Html.a
        (Styles.pipeline isHovered
            ++ [ href <|
                    Routes.toString <|
                        Routes.Pipeline { id = pipelineId, groups = [] }
               , onMouseEnter <| Hover <| Just <| SideBarPipeline pipelineId
               , onMouseLeave <| Hover Nothing
               ]
        )
        [ Html.text p.name ]
