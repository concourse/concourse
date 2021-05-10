module Resource.Causality exposing
    ( Entity
    , NodeId
    , NodeMetadata
    , NodeType(..)
    , buildGraph
    , viewCausality
    )

import Arborist.Tree as Tree
import Color
import Concourse
import Dict
import Force
import Graph exposing (Edge, Graph, Node, NodeContext, NodeId)
import Html
import Html.Attributes exposing (style)
import TypedSvg exposing (circle, g, line, rect, svg, text_)
import TypedSvg.Attributes as Attrs exposing (fill, fontSize, pointerEvents, stroke, viewBox)
import TypedSvg.Attributes.InPx exposing (cx, cy, dx, dy, height, r, strokeWidth, width, x, x1, x2, y, y1, y2)
import TypedSvg.Core exposing (Svg, text)
import TypedSvg.Types as Types exposing (Paint(..))
import Views.DictView as DictView


type alias NodeId =
    Int


type alias NodeMetadata =
    { nodeType : NodeType
    , name : String
    , version : String
    }


type NodeType
    = BuildNode
    | ResourceVersionNode


type alias Entity =
    Force.Entity NodeId { value : NodeMetadata }


viewCausality : Concourse.CausalityResourceVersion -> Html.Html msg
viewCausality =
    buildGraph


constructResourceVersion : Concourse.CausalityResourceVersion -> Html.Html msg
constructResourceVersion rv =
    let
        children =
            if List.length rv.inputTo > 0 then
                [ Html.ul [] <| List.map constructBuild rv.inputTo ]

            else
                []
    in
    Html.li [ style "margin-left" "2em" ] <|
        Html.div
            [ style "display" "flex" ]
            [ Html.div [] [ Html.text <| rv.resourceName ++ ": " ]
            , Html.div [] [ DictView.view [] <| Dict.map (\_ -> Html.text) rv.version ]
            ]
            :: children


constructBuild : Concourse.CausalityBuild -> Html.Html msg
constructBuild (Concourse.CausalityBuildVariant b) =
    Html.li
        [ style "margin-left" "2em" ]
        (Html.div
            []
            [ Html.text <| b.jobName ++ ": #" ++ b.name ]
            :: (if List.length b.outputs > 0 then
                    [ Html.ul [] <| List.map constructResourceVersion b.outputs ]

                else
                    []
               )
        )


buildGraph : Concourse.CausalityResourceVersion -> Html.Html msg
buildGraph rv =
    Html.ul
        []
        [ constructResourceVersion rv ]
