import xml.etree.ElementTree as ET
from flask import Response, url_for
from srht.config import cfg

ORIGIN = cfg("builds.sr.ht", "origin")
RFC_822_FORMAT = "%a, %d %b %Y %H:%M:%S UTC"

def generate_feed(jobs, title, link, description):
    root = ET.Element("rss", version="2.0")
    channel = ET.SubElement(root, "channel")

    ET.SubElement(channel, "title").text = title
    ET.SubElement(channel, "link").text = link
    ET.SubElement(channel, "description").text = description
    ET.SubElement(channel, "language").text = "en"

    for job in jobs:
        element = ET.Element("item")
        title, description = f"#{job.id} ({job.status.name})", job.note
        author = job.owner.username
        url = ORIGIN + url_for(
            "jobs.job_by_id", username=job.owner.username, job_id=job.id)
        time = job.updated.strftime(RFC_822_FORMAT)
        ET.SubElement(element, "title").text = title
        if description:
            ET.SubElement(element, "description").text = description
        ET.SubElement(element, "author").text = author
        ET.SubElement(element, "link").text = url
        ET.SubElement(element, "guid").text = url
        ET.SubElement(element, "pubDate").text = time
        channel.append(element)

    xml = ET.tostring(root, encoding="UTF-8")
    return Response(xml, mimetype='application/rss+xml')
